import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
export default defineConfig({plugins:[react()],server:{proxy:{'/v1':{target:process.env.API_URL||'http://localhost:8080'},'/readyz':{target:process.env.API_URL||'http://localhost:8080'}}}});
