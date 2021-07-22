package main

import (
	"context"
	"fmt"
	"net/http"

	"cloud.google.com/go/compute/metadata"
	"github.com/rs/xid"
	"github.com/treeder/gcputils"
	"github.com/treeder/goapibase"
	"github.com/treeder/gotils/v2"
)

func init() {
	fmt.Println("INIT YOOOOO")
	onGCE := metadata.OnGCE()
	fmt.Println("ON GCE???", onGCE)
	if onGCE {
		x, _ := metadata.InstanceAttributes()
		fmt.Printf("InstanceAttributes: %+v\n", x)
		s, _ := metadata.InstanceID()
		fmt.Printf("InstanceID: %+v\n", s)
		s, _ = metadata.InstanceName()
		fmt.Printf("InstanceName: %+v\n", s)
		x, _ = metadata.InstanceTags()
		fmt.Printf("InstanceTags: %+v\n", x)
		s, _ = metadata.Zone()
		fmt.Printf("InstanceZone: %+v\n", s)
		// From what I can see, instanceID will be empty if on cloud run
		// s, _ := metadata.InstanceID()
		// if s == "" {
		// 	onCloudRun = true
		// }
	}
}

func main() {
	ctx := context.Background()
	// l2 := gcputils.Info()
	// ctx = context.WithValue(ctx, "logger", l2)

	r := goapibase.InitRouter(ctx)
	// Setup your routes
	r.Use(SetupCtx)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		L(ctx).Info().Println("this is info yo")
		L(ctx).Error().Println("this is an error")
		w.Write([]byte("welcome WTF"))
	})
	// Start server
	_ = goapibase.Start(ctx, gotils.Port(8080), r)
}

func SetupCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := gcputils.With("path", r.URL.Path).F("request_id", xid.New().String())
		ctx = context.WithValue(ctx, "logger", l)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func L(ctx context.Context) gcputils.Line {
	return ctx.Value("logger").(gcputils.Line)
}

func LWith(ctx context.Context, key string, value interface{}) context.Context {
	return context.WithValue(ctx, "logger", L(ctx).With(key, value))
}
