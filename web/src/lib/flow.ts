// Laying out the desktop without scrolling: app icons fill columns from the
// left, cards fill columns from the right, each top to bottom. A column that
// passes over the dock stops above it; the others run to the bottom, so the
// room either side of the dock is used.

/** A rectangle on the page, in px from the viewport's top-left. */
export interface Box {
  left: number
  right: number
  top: number
  bottom: number
}

/** Where the dock is: only its sides and top matter. */
export interface DockBox {
  left: number
  right: number
  top: number
}

/** The lowest a column from l to r may reach. */
export function columnBottom(l: number, r: number, area: Box, dock: DockBox | null, gap: number): number {
  return dock && r > dock.left && l < dock.right ? Math.min(area.bottom, dock.top - gap) : area.bottom
}

/**
 * Cards of the given heights, in order: the first column at the right edge,
 * the next one to its left. x is the distance from the area's right edge.
 */
export function flowRight(heights: number[], area: Box, dock: DockBox | null, col: number, gap: number) {
  const pos: { x: number; y: number }[] = []
  let c = 0
  let y = 0
  for (const h of heights) {
    for (;;) {
      const x = c * (col + gap)
      const r = area.right - x
      const room = columnBottom(r - col, r, area, dock, gap) - area.top
      // A card always goes at the top of an empty column, even if it's taller.
      if (y === 0 || y + h <= room) {
        pos.push({ x, y })
        y += h + gap
        break
      }
      c++
      y = 0
    }
  }
  return { pos, cols: heights.length ? c + 1 : 0 }
}

/** n items of w × h, in order: the first column at the left edge. x is from the area's left edge. */
export function flowLeft(n: number, w: number, h: number, area: Box, dock: DockBox | null, gap: number) {
  const pos: { x: number; y: number }[] = []
  let c = 0
  let row = 0
  for (let i = 0; i < n; i++) {
    for (;;) {
      const x = c * w
      const room = columnBottom(area.left + x, area.left + x + w, area, dock, 8) - area.top
      const y = row * (h + gap)
      if (row === 0 || y + h <= room) {
        pos.push({ x, y })
        row++
        break
      }
      c++
      row = 0
    }
  }
  return { pos, cols: n ? c + 1 : 0 }
}
