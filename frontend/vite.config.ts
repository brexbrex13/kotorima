import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rolldownOptions: {
      output: {
        manualChunks(id: string) {
          if (id.includes("@tiptap") || id.includes("prosemirror"))
            return "editor";
          if (id.includes("/react/") || id.includes("/react-dom/"))
            return "react";
          return undefined;
        },
      },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
  },
});
