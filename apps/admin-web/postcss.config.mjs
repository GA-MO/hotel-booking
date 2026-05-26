// PostCSS plugin chain — Tailwind utility classes still drive layout/spacing
// for legacy pages while Mantine v9 handles the new component primitives.
// `postcss-preset-mantine` provides the rem/mixins/light-dark helpers Mantine
// stylesheets depend on. `postcss-simple-vars` resolves Mantine breakpoint
// variables ($mantine-breakpoint-sm etc) used in CSS modules.
export default {
  plugins: {
    "postcss-preset-mantine": {},
    "postcss-simple-vars": {
      variables: {
        "mantine-breakpoint-xs": "36em",
        "mantine-breakpoint-sm": "48em",
        "mantine-breakpoint-md": "62em",
        "mantine-breakpoint-lg": "75em",
        "mantine-breakpoint-xl": "88em",
      },
    },
    tailwindcss: {},
    autoprefixer: {},
  },
};
