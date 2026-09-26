package render

import (
	"bufio"
	"fmt"
	"image"
	"io"
)

// WritePPM writes img as a binary PPM (P6). PPM needs no external
// dependencies and every image tool reads it, which makes it a good default
// for a renderer.
func WritePPM(w io.Writer, img *image.RGBA) error {
	b := img.Bounds()
	bw := bufio.NewWriter(w)
	if _, err := fmt.Fprintf(bw, "P6\n%d %d\n255\n", b.Dx(), b.Dy()); err != nil {
		return err
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := img.PixOffset(x, y)
			if _, err := bw.Write(img.Pix[i : i+3]); err != nil {
				return err
			}
		}
	}
	return bw.Flush()
}
