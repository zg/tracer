package render

import (
	"bytes"
	"context"
	"image"
	"math"
	"testing"

	"github.com/zg/tracer/scene"
	"github.com/zg/tracer/vec"
)

func testWorld() (scene.Surface, *scene.Camera) {
	world := scene.List{
		scene.Sphere{Center: vec.New(0, 0, -2), Radius: 0.5,
			Material: scene.Lambertian{Albedo: vec.New(0.7, 0.3, 0.3)}},
		scene.Sphere{Center: vec.New(0, -100.5, -2), Radius: 100,
			Material: scene.Lambertian{Albedo: vec.New(0.8, 0.8, 0)}},
	}
	cam := scene.NewCamera(scene.CameraOptions{
		LookFrom: vec.New(0, 0, 0), LookAt: vec.New(0, 0, -1),
		VFOVDegrees: 90, AspectRatio: 2,
	})
	return world, cam
}

func TestRenderProducesImage(t *testing.T) {
	world, cam := testWorld()
	img, err := Render(context.Background(), world, cam, Options{
		Width: 40, Height: 20, Samples: 4, MaxDepth: 4, UseSky: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds(); got.Dx() != 40 || got.Dy() != 20 {
		t.Fatalf("bounds = %v, want 40x20", got)
	}
	// Every pixel must be opaque and actually written.
	for y := range 20 {
		for x := range 40 {
			if _, _, _, a := img.At(x, y).RGBA(); a == 0 {
				t.Fatalf("pixel (%d,%d) is transparent — not written", x, y)
			}
		}
	}
}

// Rendering twice with the same seed must give an identical image. Workers
// race over rows, so any per-worker RNG state would show up here.
func TestRenderIsDeterministic(t *testing.T) {
	world, cam := testWorld()
	opts := Options{Width: 32, Height: 16, Samples: 8, MaxDepth: 5, UseSky: true, Seed: 99}

	first, err := Render(context.Background(), world, cam, opts)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		again, err := Render(context.Background(), world, cam, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first.Pix, again.Pix) {
			t.Fatal("renders with the same seed differ")
		}
	}
}

// Worker count must not affect the result — otherwise the render depends on
// the machine it runs on.
func TestRenderIndependentOfWorkerCount(t *testing.T) {
	world, cam := testWorld()
	base := Options{Width: 24, Height: 12, Samples: 6, MaxDepth: 4, UseSky: true, Seed: 5}

	base.Workers = 1
	one, err := Render(context.Background(), world, cam, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{2, 4, 8} {
		base.Workers = n
		got, err := Render(context.Background(), world, cam, base)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(one.Pix, got.Pix) {
			t.Fatalf("%d workers produced a different image than 1 worker", n)
		}
	}
}

func TestRenderCancellation(t *testing.T) {
	world, cam := testWorld()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := Render(ctx, world, cam, Options{
		Width: 200, Height: 200, Samples: 64, MaxDepth: 16, UseSky: true,
	})
	if err == nil {
		t.Error("a cancelled context should be reported")
	}
}

func TestOptionsDefaults(t *testing.T) {
	o := Options{}.withDefaults()
	if o.Width <= 0 || o.Height <= 0 || o.Samples <= 0 || o.MaxDepth <= 0 || o.Workers <= 0 {
		t.Errorf("defaults left a field unset: %+v", o)
	}
}

// An empty world means every ray escapes, so with the sky on the image must
// be the gradient and contain no black pixels.
func TestEmptyWorldRendersSky(t *testing.T) {
	cam := scene.NewCamera(scene.CameraOptions{
		LookFrom: vec.New(0, 0, 0), LookAt: vec.New(0, 0, -1), AspectRatio: 1,
	})
	img, err := Render(context.Background(), scene.List{}, cam, Options{
		Width: 16, Height: 16, Samples: 2, MaxDepth: 4, UseSky: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for y := range 16 {
		for x := range 16 {
			r, g, b, _ := img.At(x, y).RGBA()
			if r == 0 && g == 0 && b == 0 {
				t.Fatalf("pixel (%d,%d) is black; the sky should be visible", x, y)
			}
		}
	}
}

// A light in an otherwise black scene must actually brighten the image.
func TestEmissiveMaterialLightsScene(t *testing.T) {
	world := scene.List{
		scene.Sphere{Center: vec.New(0, 0, -2), Radius: 1,
			Material: scene.DiffuseLight{Colour: vec.New(4, 4, 4)}},
	}
	cam := scene.NewCamera(scene.CameraOptions{
		LookFrom: vec.New(0, 0, 0), LookAt: vec.New(0, 0, -1), AspectRatio: 1, VFOVDegrees: 90,
	})
	img, err := Render(context.Background(), world, cam, Options{
		Width: 16, Height: 16, Samples: 4, MaxDepth: 4, UseSky: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The centre pixel looks straight at the light.
	r, g, b, _ := img.At(8, 8).RGBA()
	if r == 0 && g == 0 && b == 0 {
		t.Error("the centre pixel should be lit by the emissive sphere")
	}
}

// NaNs anywhere in the pipeline must not reach the output as noise.
func TestChannelHandlesNaNAndRange(t *testing.T) {
	if got := channel(math.NaN()); got != 0 {
		t.Errorf("channel(NaN) = %d, want 0", got)
	}
	if got := channel(-5); got != 0 {
		t.Errorf("channel(-5) = %d, want 0", got)
	}
	if got := channel(1); got < 250 {
		t.Errorf("channel(1) = %d, want near 255", got)
	}
	// Gamma correction must brighten midtones: sqrt(0.25) = 0.5.
	if got := channel(0.25); got < 120 || got > 136 {
		t.Errorf("channel(0.25) = %d, want ~128 after gamma 2", got)
	}
}

func TestWritePPM(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Pix = []uint8{
		1, 2, 3, 255, 4, 5, 6, 255,
		7, 8, 9, 255, 10, 11, 12, 255,
	}
	var buf bytes.Buffer
	if err := WritePPM(&buf, img); err != nil {
		t.Fatal(err)
	}
	want := append([]byte("P6\n2 2\n255\n"), 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("PPM output = %v, want %v", buf.Bytes(), want)
	}
}

func BenchmarkRender(b *testing.B) {
	world, cam := testWorld()
	opts := Options{Width: 80, Height: 45, Samples: 4, MaxDepth: 8, UseSky: true, Seed: 1}
	b.ResetTimer()
	for b.Loop() {
		if _, err := Render(context.Background(), world, cam, opts); err != nil {
			b.Fatal(err)
		}
	}
}
