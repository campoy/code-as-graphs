package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"github.com/pkg/errors"

	"github.com/golang/protobuf/proto"

	"golang.org/x/tools/go/packages"
	"google.golang.org/grpc"
)

func main() {
	flag.Parse()

	cfg := &packages.Config{Mode: packages.NeedImports}
	pkgs, err := packages.Load(cfg, flag.Args()...)
	if err != nil {
		log.Fatalf("load: %v\n", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		log.Printf("some errors found, but continuing")
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
			<imports>: [uid] @reverse .

			type Package {
				id: string
				imports: [Package]
				<~imports>: [Package]
			}
		`,
	}); err != nil {
		log.Fatal(err)
	}

	for _, pkg := range pkgs {
		var nquads []*api.NQuad

		nquads = append(nquads, &api.NQuad{
			Subject:   "uid(pkg)",
			Predicate: "id",
			ObjectValue: &api.Value{
				Val: &api.Value_StrVal{StrVal: pkg.ID},
			},
		})

		nquads = append(nquads, &api.NQuad{
			Subject:   "uid(pkg)",
			Predicate: "dgraph.type",
			ObjectValue: &api.Value{
				Val: &api.Value_StrVal{StrVal: "Package"},
			},
		})

		for _, pkg := range pkg.Imports {
			uid, err := upsert(dg, "id", pkg.ID, "Package")
			if err != nil {
				log.Fatalf("couldn't find package for %s: %v", pkg.ID, err)
			}
			nquads = append(nquads, &api.NQuad{
				Subject:   "uid(pkg)",
				Predicate: "imports",
				ObjectId:  uid,
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

func upsert(dg *dgo.Dgraph, pred, val, typeName string) (string, error) {
	ctx := context.Background()

	txn := dg.NewTxn()
	defer txn.Commit(ctx)

	res, err := txn.Query(ctx, fmt.Sprintf(`{q(func: eq(%s, %q)) {uid}}`, pred, val))
	if err != nil {
		return "", errors.Wrapf(err, "could not query for for %s %q", pred, val)
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
				Predicate:   pred,
				ObjectValue: &api.Value{Val: &api.Value_StrVal{StrVal: val}},
			},
			{
				Subject:     "_:f",
				Predicate:   "dgraph.type",
				ObjectValue: &api.Value{Val: &api.Value_StrVal{StrVal: typeName}},
			},
		},
	})
	if err != nil {
		return "", errors.Wrapf(err, "could not create new %s", pred)
	}
	return res.Uids["f"], nil
}
