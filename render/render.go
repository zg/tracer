// Package render turns a scene into an image by path tracing.
package render

import (
	"context"
	"image"
	"image/color"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/zg/tracer/scene"
	"github.com/zg/tracer/vec"
)

// Options configures a render.
type Options struct {
	Width, Height int
	Samples       int      // rays per pixel; noise falls as 1/sqrt(samples)
	MaxDepth      int      // bounce limit, to bound work on a shiny scene
	Background    vec.Vec3 // used when a ray escapes; zero means the sky gradient
	UseSky        bool
	Workers       int   // 0 uses GOMAXPROCS
	Seed          int64 // 0 uses a fixed seed, so renders are reproducible
}

// withDefaults fills in unset fields.
func (o Options) withDefaults() Options {
	if o.Width <= 0 {
		o.Width = 400
	}
	if o.Height <= 0 {
		o.Height = 225
	}
	if o.Samples <= 0 {
		o.Samples = 32
	}
	if o.MaxDepth <= 0 {
		o.MaxDepth = 16
	}
	if o.Workers <= 0 {
		o.Workers = runtime.GOMAXPROCS(0)
	}
	return o
}

// Render traces the scene and returns the image.
//
// Rows are distributed across workers. Each worker owns its own *rand.Rand
// seeded from the row index: sharing one generator would serialise the
// workers on its mutex, and seeding from the clock would make renders
// unreproducible.
func Render(ctx context.Context, world scene.Surface, cam *scene.Camera, opts Options) (*image.RGBA, error) {
	opts = opts.withDefaults()
	img := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))

	var nextRow atomic.Int64
	var wg sync.WaitGroup
	for range opts.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				y := int(nextRow.Add(1) - 1)
				if y >= opts.Height {
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
				}
				// Seeding per row keeps the image identical no matter how
				// rows happen to be distributed across workers.
				rng := rand.New(rand.NewSource(opts.Seed + int64(y)*9781))
				renderRow(img, world, cam, opts, y, rng)
			}
		}()
	}
	wg.Wait()

	return img, ctx.Err()
}

func renderRow(img *image.RGBA, world scene.Surface, cam *scene.Camera, opts Options, y int, rng *rand.Rand) {
	for x := range opts.Width {
		var sum vec.Vec3
		for range opts.Samples {
			// Jitter within the pixel: this is the anti-aliasing.
			u := (float64(x) + rng.Float64()) / float64(opts.Width-1)
			v := (float64(y) + rng.Float64()) / float64(opts.Height-1)
			r := cam.Ray(u, v, rng)
			sum = sum.Add(rayColour(r, world, opts, opts.MaxDepth, rng))
		}
		c := sum.Scale(1 / float64(opts.Samples))
		// Image rows run top-down but the camera's v axis runs bottom-up.
		img.Set(x, opts.Height-1-y, toRGBA(c))
	}
}

// rayColour follows a ray through the scene, accumulating attenuation until
// it escapes, is absorbed, or runs out of bounces.
func rayColour(r scene.Ray, world scene.Surface, opts Options, depth int, rng *rand.Rand) vec.Vec3 {
	// Iterative rather than recursive: depth can be large, and each bounce
	// multiplies attenuation, so a loop carrying the product is equivalent
	// and cannot overflow the stack.
	throughput := vec.New(1, 1, 1)
	acc := vec.Vec3{}

	for range depth {
		// tMin is a small epsilon, not zero: a scattered ray starts exactly
		// on the surface it just hit, and floating point error would
		// otherwise let it immediately re-hit the same point. That is the
		// classic "shadow acne" artefact.
		h, ok := world.Hit(r, 1e-3, math.Inf(1))
		if !ok {
			return acc.Add(throughput.Mul(background(r, opts)))
		}

		acc = acc.Add(throughput.Mul(h.Material.Emitted()))

		out, attenuation, scattered := h.Material.Scatter(r, h, rng)
		if !scattered {
			return acc
		}
		throughput = throughput.Mul(attenuation)
		r = out
	}
	// Bounce limit reached: no more light can be gathered along this path.
	return acc
}

func background(r scene.Ray, opts Options) vec.Vec3 {
	if !opts.UseSky {
		return opts.Background
	}
	t := 0.5 * (r.Dir.Unit().Y + 1)
	return vec.Lerp(vec.New(1, 1, 1), vec.New(0.5, 0.7, 1.0), t)
}

// toRGBA converts linear colour to 8-bit sRGB-ish output.
func toRGBA(c vec.Vec3) color.RGBA {
	// Gamma 2 correction. Without it, renders look markedly too dark,
	// because the accumulated values are linear but displays are not.
	return color.RGBA{
		R: channel(c.X),
		G: channel(c.Y),
		B: channel(c.Z),
		A: 255,
	}
}

func channel(v float64) uint8 {
	if math.IsNaN(v) {
		return 0
	}
	v = math.Sqrt(math.Max(0, v)) // gamma 2
	return uint8(256 * math.Min(math.Max(v, 0), 0.999))
}
