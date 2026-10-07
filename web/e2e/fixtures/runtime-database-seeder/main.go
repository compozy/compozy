package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/testutil/storeseed"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) (err error) {
	flags := flag.NewFlagSet("runtime-database-seeder", flag.ContinueOnError)
	output := flags.String("output", "", "closed database template path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *output == "" {
		return errors.New("runtime database seeder requires --output")
	}
	seed, err := storeseed.NewCombined(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, seed.Close()) }()
	return seed.Clone(*output)
}
