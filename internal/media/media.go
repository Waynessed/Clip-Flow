package media

import (
	"clipflow/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Probe(ctx context.Context, path string) (model.Media, error) {
	m, err := Inspect(ctx, path)
	if err != nil {
		return m, err
	}
	if m.Duration > 30 {
		return m, fmt.Errorf("video duration must be at most 30 seconds")
	}
	return m, nil
}

// Inspect validates actual MP4 metadata without applying the upload duration
// limit: generated AAC previews may have a small amount of encoder padding.
func Inspect(ctx context.Context, path string) (model.Media, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-show_streams", "-show_format", "-of", "json", path).Output()
	if err != nil {
		return model.Media{}, fmt.Errorf("cannot inspect MP4 video: %w", err)
	}
	var p struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		}
		Format struct {
			Duration   string
			FormatName string            `json:"format_name"`
			Tags       map[string]string `json:"tags"`
		}
	}
	if err = json.Unmarshal(out, &p); err != nil {
		return model.Media{}, err
	}
	duration, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return model.Media{}, fmt.Errorf("video duration must be finite and greater than zero")
	}
	brand := strings.TrimSpace(p.Format.Tags["major_brand"])
	if !strings.Contains(p.Format.FormatName, "mp4") || !(strings.HasPrefix(brand, "mp4") || strings.HasPrefix(brand, "iso") || brand == "avc1" || brand == "MSNV" || brand == "M4V") {
		return model.Media{}, fmt.Errorf("supported MP4 container required")
	}
	m := model.Media{Duration: duration}
	for _, s := range p.Streams {
		if s.CodecType == "video" && m.Width == 0 {
			m.Width = s.Width
			m.Height = s.Height
		}
		if s.CodecType == "audio" {
			m.Audio = true
		}
	}
	if m.Width < 2 || m.Height < 2 || m.Width > 7680 || m.Height > 4320 {
		return m, fmt.Errorf("video stream dimensions must be between 2x2 and 7680x4320")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return m, err
	}
	m.Size = fi.Size()
	return m, nil
}
func ffmpeg(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", append([]string{"-nostdin", "-v", "error", "-y", "-threads", "1", "-filter_threads", "1", "-protocol_whitelist", "file,pipe"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 2000 {
			out = out[len(out)-2000:]
		}
		return fmt.Errorf("FFmpeg: %w: %s", err, out)
	}
	return nil
}
func Process(ctx context.Context, input, dir string) (model.Manifest, error) {
	in, err := Probe(ctx, input)
	if err != nil {
		return model.Manifest{}, err
	}
	thumb := filepath.Join(dir, "thumbnail.jpg")
	preview := filepath.Join(dir, "preview.mp4")
	if err = ffmpeg(ctx, "-i", input, "-map", "0:v:0", "-frames:v", "1", "-vf", "scale=w='min(640,iw)':h=-2", "-q:v", "3", thumb); err != nil {
		return model.Manifest{}, err
	}
	if err = ffmpeg(ctx, "-i", input, "-map", "0:v:0", "-map", "0:a:0?", "-vf", "scale=w=-2:h='trunc(min(480,ih)/2)*2'", "-c:v", "libx264", "-threads", "1", "-preset", "veryfast", "-crf", "28", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "96k", "-movflags", "+faststart", preview); err != nil {
		return model.Manifest{}, err
	}
	pm, err := Inspect(ctx, preview)
	if err != nil {
		return model.Manifest{}, err
	}
	if pm.Height > 480 || pm.Width%2 != 0 || pm.Height%2 != 0 {
		return model.Manifest{}, fmt.Errorf("preview dimensions invalid")
	}
	f, err := os.Open(thumb)
	if err != nil {
		return model.Manifest{}, err
	}
	_, err = jpeg.DecodeConfig(f)
	f.Close()
	if err != nil {
		return model.Manifest{}, err
	}
	m := model.Manifest{Objects: map[string]string{}, Input: in, Preview: pm, Sizes: map[string]int64{}}
	for _, name := range []string{"thumbnail", "preview"} {
		ext := ".jpg"
		if name == "preview" {
			ext = ".mp4"
		}
		fi, err := os.Stat(filepath.Join(dir, name+ext))
		if err != nil || fi.Size() == 0 {
			return m, fmt.Errorf("missing %s output", name)
		}
		m.Sizes[name] = fi.Size()
	}
	return m, nil
}
