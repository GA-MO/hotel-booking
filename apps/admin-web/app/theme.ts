// Mantine theme — keep it minimal in Phase 2 and grow once the design system
// settles. Primary color matches the previous neutral-900 brand tone so the
// migration doesn't shift the visual identity all at once.
import { createTheme } from "@mantine/core";

export const theme = createTheme({
  primaryColor: "dark",
  fontFamily: "var(--font-sans), system-ui, sans-serif",
  defaultRadius: "md",
  cursorType: "pointer",
});
