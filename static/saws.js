function drawPencil(p) {
  const svg = document.querySelector("svg");
  const path = document.querySelector("svg > path");
  const width = svg.getAttribute("width");
  const height = svg.getAttribute("height");

  const interval = width / p;

  let d = "";
  for (let i = 0; i < p; i++) {
    if (i == 0) {
      d += `M0 ${height / 2}`;
    }

    d += ` L${interval * i + (interval * (i + 1)) / 2} 0`;
    d += ` L${interval * i + (interval * (i + 1)) / 2} ${height}`;
    d += ` L${interval * (i + 1)} ${height / 2}`;
  }
  console.log(d);
  path.setAttribute("d", d);
}

/**
 * Returns a string that can be used to draw sawtooth waves
 * @param {number} startY - y coordinate for the start of the path
 * @param {number} width - total width of the wave
 * @param {number} height - height of the wave
 * @param {number} freq - number of cycles in the wave
 * @returns {string} d - attribute to be used in <path/> tags
 */
function sawPath(startY, width, height, freq) {
  const interval = width / freq;

  let d = `M0 ${startY}`;
  for (let i = 0; i < freq; i++) {
    d += ` L${interval / 2 + interval * i} ${startY - height / 2}`;
    d += ` L${interval / 2 + interval * i} ${startY + height / 2}`;
    d += ` L${interval * (i + 1)} ${startY}`;
  }
  return d;
}

/**
 * @callback StrokeWidthFn
 * @return number
 */

/**
 * @callback HeightFn
 * @return number
 */

/**
 * @callback FreqFn
 * @return number
 */

/**
 * @callback StrokeColorFn
 * @return string
 */

/**
 * @callback IntervalYFn
 * @param {number} i - the iteration of the saw draw
 * @return {number} intervalY - y interval between each drawn saw
 */

/**
 * @callback StartYFn
 * @param {number} i - the iteration of the saw draw
 * @return {number} startY - starting y position for drawing saw set
 */

/**
 * @typedef {Object} SawDrawOpts - options for sawDraw, xFn parameters
 *  are tried first on each iteration before x and then falling back to a default.
 *
 * @property {number} [n] - the intended max number of saws drawn
 * @property {number} [strokeWidth] - the saws' stroke width
 * @property {StrokeWidthFn} [strokeWidthFn] - function that determines saw's stroke width
 * @property {string} [strokeColor] - color of the saw
 * @property {number} [startY] - y coordinate where the first saw is drawn
 * @property {StartYFn} [startYFn] - function that determines y coordinate
 * @property {number} [intervalY] - y interval between each drawn saw
 * @property {IntervalYFn} [intervalYFn] - function that determines y interval
 * @property {number} [width] - total width of the saw wave
 * @property {FreqFn} [freqFn] - function that determines frequency of the wave
 * @property {number} [height] - height of the wave
 * @property {HeightFn} [heightFn] - function that determines the height of the wave
 * @property {number} [restartY] - y coordinate at which to restart saw draw from the start
 * @property {number} [timeInterval] - time between each drawn saw
 */

/**
 * Adds saws to an svg element
 * @param {HTMLElement} svg - element to place paths under
 * @param {SawDrawOpts} [opts] - options for drawing the saws
 * @returns {number} intervalID - the intervalID so it can be cleared
 */
export function sawDraw(svg, opts) {
  // setting params with defaults
  const n = opts?.n || 20;
  const strokeWidth = opts?.strokeWidth || 4;
  const startY = opts?.startY || 100;
  const intervalY = opts?.intervalY || 40;
  const width = opts?.width || 3000;
  const height = opts?.height || 30;
  const period = opts?.period || 8;
  const restartY = opts?.restartY || 700;
  const timeInterval = opts?.timeInterval || 3000;
  const strokeColor = opts?.strokeColor || "black";

  const saws = [];
  let i = 0;
  let j = 0;

  return setInterval(() => {
    if (saws.length >= n) {
      const first = saws.shift();
      first.style.opacity = 0;
    }

    const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
    path.setAttribute("class", "sawtooth");
    path.setAttribute("stroke", strokeColor);
    path.setAttribute("fill", "none");

    path.setAttribute("stroke-width", opts?.strokeWidthFn?.() || strokeWidth);

    const startDrawY =
      (opts?.startYFn?.(i) || startY) +
      (opts?.intervalYFn?.(i) || intervalY) * j;

    path.setAttribute(
      "d",
      sawPath(
        startDrawY,
        width,
        opts?.heightFn?.() || height,
        opts?.periodFn?.() || period,
      ),
    );
    saws.push(path);

    svg.appendChild(path);

    if (startDrawY > restartY) {
      j = 0;
    } else {
      j += 1;
    }
    i += 1;
  }, timeInterval);
}
