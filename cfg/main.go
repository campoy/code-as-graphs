package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"golang.org/x/tools/go/cfg"
	"google.golang.org/grpc"
)

func main() {
	name := flag.String("n", "main", "name of the function to be analyzed")
	flag.Parse()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", os.Stdin, 0)
	if err != nil {
		log.Fatal(err)
	}

	for _, decl := range f.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if f.Name.Name != *name {
			continue
		}
		analyze(fset, f.Body)
	}
}

func mayReturn(call *ast.CallExpr) bool { return true }

func analyze(fset *token.FileSet, body *ast.BlockStmt) {
	cfg := cfg.New(body, mayReturn)

	conn, err := grpc.Dial("localhost:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	var nquads []*api.NQuad
	for _, block := range cfg.Blocks {
		nquads = append(nquads, &api.NQuad{
			Subject:     fmt.Sprintf("_:block%d", block.Index),
			Predicate:   "dgraph.type",
			ObjectValue: &api.Value{Val: &api.Value_StrVal{StrVal: "Block"}},
		})
		nquads = append(nquads, &api.NQuad{
			Subject:   fmt.Sprintf("_:block%d", block.Index),
			Predicate: "block",
			ObjectValue: &api.Value{
				Val: &api.Value_StrVal{StrVal: block.String()},
			},
		})

		for idx, node := range block.Nodes {
			id := fmt.Sprintf("_:node%d_%d", block.Index, idx)

			var buf bytes.Buffer
			if err := printer.Fprint(&buf, fset, node); err != nil {
				log.Fatalf("couldn't print node: %v", err)
			}

			nquads = append(nquads, &api.NQuad{
				Subject:   fmt.Sprintf("_:block%d", block.Index),
				Predicate: "node",
				ObjectId:  id,
				Facets: []*api.Facet{
					{Key: "number", Value: []byte(fmt.Sprint(idx))},
				},
			})

			nquads = append(nquads, &api.NQuad{
				Subject:   id,
				Predicate: "dgraph.type",
				ObjectValue: &api.Value{
					Val: &api.Value_StrVal{StrVal: "Node"},
				},
			})

			nquads = append(nquads, &api.NQuad{
				Subject:   id,
				Predicate: "body",
				ObjectValue: &api.Value{
					Val: &api.Value_StrVal{StrVal: buf.String()},
				},
			})

		}

		for _, succ := range block.Succs {
			nquads = append(nquads, &api.NQuad{
				Subject:   fmt.Sprintf("_:block%d", block.Index),
				Predicate: "succ",
				ObjectId:  fmt.Sprintf("_:block%d", succ.Index),
			})
		}
	}

	ctx := context.Background()
	dc := api.NewDgraphClient(conn)
	dg := dgo.NewDgraphClient(dc)

	if err := dg.Alter(ctx, &api.Operation{
		Schema: `
			<block>: string @index(term) .
			<node>: [uid] .
			<succ>: [uid] @reverse .
			<body>: string @index(term) .

			type Block {
				block: string
				node: [Node]
				succ: [Block]
				<~succ>: [Block]
			}

			type Node {
				<~node>: [Block]
				body: string
			}
		`,
	}); err != nil {
		log.Fatal(err)
	}

	_, err = dg.NewTxn().Mutate(ctx, &api.Mutation{Set: nquads, CommitNow: true})
	if err != nil {
		log.Fatalf("couldn't do stuff: %v", err)
	}
}
