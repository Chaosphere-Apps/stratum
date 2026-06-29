import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
export default defineConfig({
    plugins: [react()],
    build: {
        minify: 'esbuild',
        sourcemap: false,
    },
    optimizeDeps: {
        include: ['@tiptap/react', '@tiptap/starter-kit', '@tiptap/extension-link'],
    },
});
