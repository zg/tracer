// Command tracer renders a demo scene by path tracing.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/zg/tracer/render"
	"github.com/zg/tracer/scene"
	"github.com/zg/tracer/vec"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tracer:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		width   = flag.Int("w", 600, "image width in pixels")
		height  = flag.Int("h", 400, "image height in pixels")
		samples = flag.Int("samples", 64, "rays per pixel; noise falls as 1/sqrt(samples)")
		depth   = flag.Int("depth", 24, "maximum bounces per ray")
		out     = flag.String("o", "out.png", "output file (.png or .ppm)")
		sceneNm = flag.String("scene", "spheres", "scene: spheres, cornell")
		seed    = flag.Int64("seed", 1, "random seed; the same seed gives the same image")
		workers = flag.Int("workers", 0, "render workers (0 = one per CPU)")
	)
	flag.Parse()

	world, cam, useSky := buildScene(*sceneNm, float64(*width)/float64(*height), *seed)
	if world == nil {
		return fmt.Errorf("unknown scene %q", *sceneNm)
	}

	// Ctrl-C stops the render and still writes what has been traced so far,
	// which beats losing a long render entirely.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	start := time.Now()
	img, err := render.Render(ctx, world, cam, render.Options{
		Width: *width, Height: *height, Samples: *samples,
		MaxDepth: *depth, UseSky: useSky, Seed: *seed, Workers: *workers,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "render interrupted after %s, writing partial image\n", time.Since(start).Round(time.Millisecond))
	} else {
		fmt.Fprintf(os.Stderr, "rendered %dx%d at %d spp in %s\n",
			*width, *height, *samples, time.Since(start).Round(time.Millisecond))
	}

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()

	if strings.EqualFold(filepath.Ext(*out), ".ppm") {
		return render.WritePPM(f, img)
	}
	return png.Encode(f, img)
}

// buildScene returns the named scene, its camera, and whether the sky
// gradient should be used as the background.
func buildScene(name string, aspect float64, seed int64) (scene.Surface, *scene.Camera, bool) {
	switch name {
	case "spheres":
		return spheresScene(aspect, seed)
	case "cornell":
		return cornellScene(aspect)
	default:
		return nil, nil, false
	}
}

func spheresScene(aspect float64, seed int64) (scene.Surface, *scene.Camera, bool) {
	rng := rand.New(rand.NewSource(seed))
	var surfaces []scene.Surface

	surfaces = append(surfaces, scene.Sphere{
		Center: vec.New(0, -1000, 0), Radius: 1000,
		Material: scene.Lambertian{Albedo: vec.New(0.5, 0.5, 0.5)},
	})

	for a := -6; a < 6; a++ {
		for b := -6; b < 6; b++ {
			center := vec.New(float64(a)+0.9*rng.Float64(), 0.2, float64(b)+0.9*rng.Float64())
			if center.Sub(vec.New(4, 0.2, 0)).Len() <= 0.9 {
				continue
			}
			var m scene.Material
			switch choose := rng.Float64(); {
			case choose < 0.7:
				m = scene.Lambertian{Albedo: vec.New(rng.Float64()*rng.Float64(), rng.Float64()*rng.Float64(), rng.Float64()*rng.Float64())}
			case choose < 0.9:
				m = scene.Metal{Albedo: vec.New(0.5+rng.Float64()/2, 0.5+rng.Float64()/2, 0.5+rng.Float64()/2), Fuzz: rng.Float64() / 2}
			default:
				m = scene.Dielectric{RefIdx: 1.5}
			}
			surfaces = append(surfaces, scene.Sphere{Center: center, Radius: 0.2, Material: m})
		}
	}

	surfaces = append(surfaces,
		scene.Sphere{Center: vec.New(0, 1, 0), Radius: 1, Material: scene.Dielectric{RefIdx: 1.5}},
		scene.Sphere{Center: vec.New(-4, 1, 0), Radius: 1, Material: scene.Lambertian{Albedo: vec.New(0.4, 0.2, 0.1)}},
		scene.Sphere{Center: vec.New(4, 1, 0), Radius: 1, Material: scene.Metal{Albedo: vec.New(0.7, 0.6, 0.5)}},
	)

	cam := scene.NewCamera(scene.CameraOptions{
		LookFrom: vec.New(13, 2, 3), LookAt: vec.New(0, 0, 0),
		VFOVDegrees: 20, AspectRatio: aspect,
		Aperture: 0.1, FocusDistance: 10,
	})
	return scene.NewBVH(surfaces), cam, true
}

func cornellScene(aspect float64) (scene.Surface, *scene.Camera, bool) {
	white := scene.Lambertian{Albedo: vec.New(0.73, 0.73, 0.73)}
	red := scene.Lambertian{Albedo: vec.New(0.65, 0.05, 0.05)}
	green := scene.Lambertian{Albedo: vec.New(0.12, 0.45, 0.15)}

	surfaces := []scene.Surface{
		scene.Plane{Point: vec.New(-3, 0, 0), Normal: vec.New(1, 0, 0), Material: red},
		scene.Plane{Point: vec.New(3, 0, 0), Normal: vec.New(-1, 0, 0), Material: green},
		scene.Plane{Point: vec.New(0, 0, 0), Normal: vec.New(0, 1, 0), Material: white},
		scene.Plane{Point: vec.New(0, 6, 0), Normal: vec.New(0, -1, 0), Material: white},
		scene.Plane{Point: vec.New(0, 0, -6), Normal: vec.New(0, 0, 1), Material: white},
		scene.Sphere{Center: vec.New(0, 5.9, -3), Radius: 1.2,
			Material: scene.DiffuseLight{Colour: vec.New(6, 6, 6)}},
		scene.Sphere{Center: vec.New(-1.2, 1, -3.5), Radius: 1, Material: scene.Dielectric{RefIdx: 1.5}},
		scene.Sphere{Center: vec.New(1.3, 1, -2.5), Radius: 1,
			Material: scene.Metal{Albedo: vec.New(0.8, 0.85, 0.88), Fuzz: 0.05}},
	}

	cam := scene.NewCamera(scene.CameraOptions{
		LookFrom: vec.New(0, 3, 6), LookAt: vec.New(0, 2.5, -3),
		VFOVDegrees: 55, AspectRatio: aspect,
	})
	return scene.NewBVH(surfaces), cam, false
}
