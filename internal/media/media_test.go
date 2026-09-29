package media

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDurationLimitAppliesToInputNotAACPadding(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("requires real FFmpeg")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	input := filepath.Join(dir, "boundary.mp4")
	out, err := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=32x32:r=1", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "30", "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", input).CombinedOutput()
	if err != nil {
		t.Fatalf("generate boundary clip: %v %s", err, out)
	}
	in, err := Probe(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if in.Duration != 30 {
		t.Fatalf("boundary fixture duration %f", in.Duration)
	}
	m, err := Process(ctx, input, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Preview.Audio || m.Preview.Duration < 30 || m.Preview.Duration > 30.1 {
		t.Fatalf("bad preview: %+v", m.Preview)
	}
	t.Logf("accepted input %.3fs; real AAC preview %.3fs", in.Duration, m.Preview.Duration)
	tooLong := filepath.Join(dir, "long.mp4")
	out, err = exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=32x32:r=1", "-t", "31", "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p", tooLong).CombinedOutput()
	if err != nil {
		t.Fatalf("generate long clip: %v %s", err, out)
	}
	if _, err = Probe(ctx, tooLong); err == nil {
		t.Fatal("31-second input accepted")
	}
}
