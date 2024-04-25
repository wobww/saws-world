const colors = require("tailwindcss/colors");

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/*.html"],
  theme: {
    colors: {
      bg: {
        100: "hsl(60 6.7% 97.1%)",
        200: "hsl(50 23.1% 94.9%)",
        300: "hsl(49 26.8% 92%)",
        400: "hsl(49 25.8% 87.8%)",
        500: "hsl(46 28.3% 82%)",
        600: "hsl(47 27% 71%)",
      },
      bilbao: {
        50: "#effbea",
        100: "#dcf5d2",
        200: "#bbecaa",
        300: "#90de78",
        400: "#6acd4e",
        500: "#4ab32f",
        600: "#368e22",
        700: "#317b22",
        800: "#27571d",
        900: "#234a1d",
        950: "#0e280b",
      },
      mine: {
        50: "#f6f6f6",
        100: "#e7e7e7",
        200: "#d1d1d1",
        300: "#b0b0b0",
        400: "#888888",
        500: "#6d6d6d",
        600: "#5d5d5d",
        700: "#4f4f4f",
        800: "#454545",
        900: "#333333",
        950: "#262626",
      },

      ...colors,
    },
    fontFamily: {
      sans: ["Montserrat", "sans-serif"],
      mono: ["Roboto Mono", "ui-monospace"],
    },
    extend: {},
  },
  plugins: [],
};
