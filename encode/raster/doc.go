// Package raster renders display boxes to RGBA images and PNG streams with a
// pure-Go antialiased vector rasterizer.
//
// Encode and EncodePNG walk the visible display-list commands in paint order.
// Path templates resolve as At + point*Em, rules fill their placed rectangles,
// explicit paints override palette roles, and Box.Bounds includes certified
// overflow. A nil Options uses DefaultOptions.
package raster
