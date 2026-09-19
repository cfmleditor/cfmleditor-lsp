// The viewer's D3 surface, and nothing else. Full d3 is 280KB minified; the eight
// modules the four views actually use are a fraction of that, and the bundle is
// embedded in every generated HTML file, so the difference is paid per report.
export {
  select, selectAll,
} from "d3-selection";
export {
  hierarchy, cluster, pack,
} from "d3-hierarchy";
export {
  lineRadial, curveBundle,
} from "d3-shape";
export {
  forceSimulation, forceLink, forceManyBody, forceCollide, forceX, forceY,
} from "d3-force";
export { zoom, zoomIdentity } from "d3-zoom";
export { drag } from "d3-drag";
export { quadtree } from "d3-quadtree";
export {
  rollup, groups, ascending, descending, max, min,
} from "d3-array";
