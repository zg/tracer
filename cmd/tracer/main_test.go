package main

import (
	"context"
	"testing"

	"github.com/zg/tracer/render"
)

func TestBuildScenes(t *testing.T) {
	for _, name := range []string{"spheres", "cornell"} {
		t.Run(name, func(t *testing.T) {
			world, cam, _ := buildScene(name, 1.5, 1)
			if world == nil || cam == nil {
				t.Fatal("scene or camera is nil")
			}
			// A tiny render proves the scene is actually traceable, not
			// just that it was constructed.
			img, err := render.Render(context.Background(), world, cam, render.Options{
				Width: 12, Height: 8, Samples: 2, MaxDepth: 3, Seed: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != 12 {
				t.Fatalf("bounds = %v", img.Bounds())
			}
		})
	}
}

func TestBuildSceneUnknown(t *testing.T) {
	if world, _, _ := buildScene("nope", 1.5, 1); world != nil {
		t.Error("unknown scene should return nil")
	}
}
