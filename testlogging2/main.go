package main

import (
	"context"

	"github.com/treeder/gcputils"
	"github.com/treeder/gotils/v2"
)

func main() {
	ctx := context.Background()
	err := yo(ctx)
	gcputils.Error().Printf("dang: %v", err)
}

func yo(ctx context.Context) error {
	ctx = gotils.With(ctx, "yo", "dawg")
	return gotils.C(ctx).Errorf("error1")
}
