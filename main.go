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
	"github.com/pkg/errors"

	"github.com/golang/protobuf/proto"

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

	// TODO: add timeout
	ctx := context.Background()
	if err := dg.Alter(ctx, &api.Operation{
		Schema: `
			<id>: string @index(exact) .
			<go-files>: [uid] .
			<path>: string @index(hash) .
		`,
	}); err != nil {
		log.Fatal(err)
	}

	// Print the names of the source files
	// for each package listed on the command line.
	for _, pkg := range pkgs {
		var nquads []*api.NQuad

		nquads = append(nquads, &api.NQuad{
			Subject:   "uid(pkg)",
			Predicate: "id",
			ObjectValue: &api.Value{
				Val: &api.Value_StrVal{StrVal: pkg.ID},
			},
		})

		for _, f := range pkg.GoFiles {
			file, err := findFile(dg, f)
			if err != nil {
				log.Printf("couldn't find file for %s: %v", f, err)
				log.Fatal(err)
			}
			nquads = append(nquads, &api.NQuad{
				Subject:   "uid(pkg)",
				Predicate: "go-files",
				ObjectId:  file,
			})
		}

		req := &api.Request{
			Query: fmt.Sprintf(`{q(func: eq(id, %q)) { pkg as uid }}`, pkg.ID),
			Mutations: []*api.Mutation{
				{
					Set: nquads,
				},
			},
			CommitNow: true,
		}
		fmt.Println(proto.MarshalTextString(req))
		res, err := dg.NewTxn().Do(ctx, req)
		if err != nil {
			log.Fatalf("couldn't do stuff: %v", err)
		}
		fmt.Printf("%s", res.GetJson())

	}

}

func findFile(dg *dgo.Dgraph, path string) (string, error) {
	ctx := context.Background()

	txn := dg.NewTxn()
	defer txn.Commit(ctx)

	res, err := txn.Query(ctx, fmt.Sprintf(`{q(func: eq(path, %q)) {uid}}`, path))
	if err != nil {
		return "", errors.Wrap(err, "could not query for file path")
	}

	var data struct{ Q []struct{ UID string } }
	if err := json.Unmarshal(res.GetJson(), &data); err != nil {
		return "", errors.Wrap(err, "could not parse response")
	}

	if len(data.Q) > 0 {
		return data.Q[0].UID, nil
	}

	res, err = txn.Mutate(ctx, &api.Mutation{
		Set: []*api.NQuad{
			{
				Subject:     "_:f",
				Predicate:   "path",
				ObjectValue: &api.Value{Val: &api.Value_StrVal{StrVal: path}},
			},
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "could not create new file")
	}
	return res.Uids["f"], nil
}
