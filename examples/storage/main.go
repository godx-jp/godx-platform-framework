// Demonstrates the storage module with the Laravel-style multi-disk
// API. Configure two disks via env (one local, one in-memory), write
// some objects, list directories, take a public URL, and shut down
// cleanly.
//
// Run with the default config (single local disk under ./storage):
//
//	go run ./examples/storage
//
// Or declare multiple disks explicitly:
//
//	STORAGE_DEFAULT_DISK=uploads \
//	STORAGE_DISKS=uploads,cache \
//	STORAGE_DISK_UPLOADS_DRIVER=local \
//	STORAGE_DISK_UPLOADS_ROOT=/tmp/example-uploads \
//	STORAGE_DISK_UPLOADS_VISIBILITY=public \
//	STORAGE_DISK_UPLOADS_PUBLIC_URL=https://cdn.example.com \
//	STORAGE_DISK_CACHE_DRIVER=memory \
//	go run ./examples/storage
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/storage"
	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// When run from the framework repo, the default "./storage" root
	// collides with our own source tree. Use a per-run temp dir unless
	// the caller has explicitly configured a root.
	if os.Getenv("STORAGE_DISK_LOCAL_ROOT") == "" && os.Getenv("STORAGE_DISKS") == "" {
		tmp, err := os.MkdirTemp("", "storage-example-*")
		if err != nil {
			log.Fatalf("temp dir: %v", err)
		}
		os.Setenv("STORAGE_DISK_LOCAL_ROOT", tmp)
		defer os.RemoveAll(tmp)
	}

	app := framework.New("storage-example", "0.6.0").Use(storage.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() {
		if err := app.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	mgr, ok := storage.FromApp(app)
	if !ok {
		log.Fatal("storage module did not publish a manager")
	}

	fmt.Printf("registered disks: %v\n", mgr.Disks())
	fmt.Printf("default disk: %q\n", mgr.DefaultName())

	def, _ := mgr.Default()
	if err := def.Put(ctx, "greetings/hello.txt", []byte("world")); err != nil {
		log.Fatalf("put: %v", err)
	}
	body, err := def.Get(ctx, "greetings/hello.txt")
	if err != nil {
		log.Fatalf("get: %v", err)
	}
	fmt.Printf("read back: %q\n", body)

	files, _ := def.Files(ctx, "greetings")
	dirs, _ := def.Directories(ctx, "")
	fmt.Printf("files under greetings/: %v\n", files)
	fmt.Printf("top-level dirs:         %v\n", dirs)

	if err := def.Put(ctx, "public.jpg", []byte("pretend image data"),
		storage.WithContentType("image/jpeg"),
		storage.WithVisibility(driver.VisibilityPublic),
	); err != nil {
		log.Fatalf("put public: %v", err)
	}
	if u, err := def.URL("public.jpg"); err == nil {
		fmt.Printf("public URL: %s\n", u)
	} else if errors.Is(err, driver.ErrNotSupported) {
		fmt.Println("public URL: driver has no URL surface (set STORAGE_DISK_<NAME>_PUBLIC_URL to enable)")
	}

	// Cross-disk copy if a second disk is configured.
	if other, ok := mgr.Disk("cache"); ok {
		fmt.Println("copying greetings/hello.txt → cache disk")
		if err := other.Put(ctx, "greetings/hello.txt", body); err != nil {
			log.Printf("cache put: %v", err)
		}
	}

	if root := os.Getenv("STORAGE_DISK_LOCAL_ROOT"); root != "" {
		fmt.Printf("local disk root: %s\n", root)
	}
}
