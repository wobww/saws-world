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
    },
    fontFamily: {
      sans: ["Montserrat", "sans-serif"],
      mono: ["Roboto Mono", "ui-monospace"],
    },
    extend: {},
  },
  plugins: [],
};
