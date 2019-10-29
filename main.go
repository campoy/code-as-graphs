package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"golang.org/x/tools/go/packages"
	"google.golang.org/grpc"
)

func main() {
	flag.Parse()

	cfg := &packages.Config{Mode: packages.NeedFiles | packages.NeedSyntax}
	pkgs, err := packages.Load(cfg, flag.Args()...)
	if err != nil {
		log.Fatalf("load: %v\n", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}

	conn, err := grpc.Dial("localhost:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
	dg := dgo.NewDgraphClient(dc)

	// Print the names of the source files
	// for each package listed on the command line.
	for _, pkg := range pkgs {
		type GoFile struct {
			Path string `json:"path"`
		}
		var files = struct {
			ID      string   `json:"id"`
			GoFiles []GoFile `json:"go-files"`
		}{ID: pkg.ID}

		for _, path := range pkg.GoFiles {
			files.GoFiles = append(files.GoFiles, GoFile{path})
		}

		b, err := json.Marshal(files)
		if err != nil {
			log.Fatal(err)
		}

		// TODO: add timeout
		ctx := context.Background()
		dg.NewTxn().Mutate(ctx, &api.Mutation{
			SetJson:   b,
			CommitNow: true,
		})
		fmt.Println(pkg.ID, pkg.GoFiles)
	}

}
