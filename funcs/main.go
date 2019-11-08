package main

import (
	"flag"
	"fmt"
	"go/ast"
	"log"

	"golang.org/x/tools/go/packages"
)

func main() {
	flag.Parse()

	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedSyntax}
	pkgs, err := packages.Load(cfg, flag.Args()...)
	if err != nil {
		log.Fatalf("load: %v\n", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		log.Printf("some errors found, but continuing")
	}

	// 	conn, err := grpc.Dial("localhost:9080", grpc.WithInsecure())
	// 	if err != nil {
	// 		log.Fatal("While trying to dial gRPC")
	// 	}
	// 	defer conn.Close()

	// 	dc := api.NewDgraphClient(conn)
	// 	dg := dgo.NewDgraphClient(dc)

	// 	// TODO: add timeout
	// 	ctx := context.Background()
	// 	if err := dg.Alter(ctx, &api.Operation{
	// 		Schema: `
	// 			<id>: string @index(exact) .
	// 			<imports>: [uid] @reverse .

	// 			type Package {
	// 				id: string
	// 				imports: [Package]
	// 				<~imports>: [Pacakge]
	// 			}
	// 		`,
	// 	}); err != nil {
	// 		log.Fatal(err)
	// 	}

	fmt.Println(pkgs)
	for _, pkg := range pkgs {
		fmt.Println(pkg.Syntax)
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				f, ok := node.(*ast.FuncDecl)
				if !ok {
					return true
				}
				analyzeFunc(pkg, f)
				return false
			})
		}
	}
	// 		var nquads []*api.NQuad

	// 		nquads = append(nquads, &api.NQuad{
	// 			Subject:   "uid(pkg)",
	// 			Predicate: "id",
	// 			ObjectValue: &api.Value{
	// 				Val: &api.Value_StrVal{StrVal: pkg.ID},
	// 			},
	// 		})

	// 		nquads = append(nquads, &api.NQuad{
	// 			Subject:   "uid(pkg)",
	// 			Predicate: "dgraph.type",
	// 			ObjectValue: &api.Value{
	// 				Val: &api.Value_StrVal{StrVal: "Package"},
	// 			},
	// 		})

	// 		for _, pkg := range pkg.Imports {
	// 			uid, err := upsert(dg, "id", pkg.ID, "Package")
	// 			if err != nil {
	// 				log.Fatalf("couldn't find package for %s: %v", pkg.ID, err)
	// 			}
	// 			nquads = append(nquads, &api.NQuad{
	// 				Subject:   "uid(pkg)",
	// 				Predicate: "imports",
	// 				ObjectId:  uid,
	// 			})
	// 		}

	// 		req := &api.Request{
	// 			Query: fmt.Sprintf(`{q(func: eq(id, %q)) { pkg as uid }}`, pkg.ID),
	// 			Mutations: []*api.Mutation{
	// 				{
	// 					Set: nquads,
	// 				},
	// 			},
	// 			CommitNow: true,
	// 		}
	// 		fmt.Println(proto.MarshalTextString(req))
	// 		res, err := dg.NewTxn().Do(ctx, req)
	// 		if err != nil {
	// 			log.Fatalf("couldn't do stuff: %v", err)
	// 		}
	// 		fmt.Printf("%s", res.GetJson())
	// 	}
	// }

	// func upsert(dg *dgo.Dgraph, pred, val, typeName string) (string, error) {
	// 	ctx := context.Background()

	// 	txn := dg.NewTxn()
	// 	defer txn.Commit(ctx)

	// 	res, err := txn.Query(ctx, fmt.Sprintf(`{q(func: eq(%s, %q)) {uid}}`, pred, val))
	// 	if err != nil {
	// 		return "", errors.Wrapf(err, "could not query for for %s %q", pred, val)
	// 	}

	// 	var data struct{ Q []struct{ UID string } }
	// 	if err := json.Unmarshal(res.GetJson(), &data); err != nil {
	// 		return "", errors.Wrap(err, "could not parse response")
	// 	}

	// 	if len(data.Q) > 0 {
	// 		return data.Q[0].UID, nil
	// 	}

	// 	res, err = txn.Mutate(ctx, &api.Mutation{
	// 		Set: []*api.NQuad{
	// 			{
	// 				Subject:     "_:f",
	// 				Predicate:   pred,
	// 				ObjectValue: &api.Value{Val: &api.Value_StrVal{StrVal: val}},
	// 			},
	// 			{
	// 				Subject:     "_:f",
	// 				Predicate:   "dgraph.type",
	// 				ObjectValue: &api.Value{Val: &api.Value_StrVal{StrVal: typeName}},
	// 			},
	// 		},
	// 	})
	// 	if err != nil {
	// 		return "", errors.Wrapf(err, "could not create new %s", pred)
	// 	}
	// 	return res.Uids["f"], nil
}

func analyzeFunc(pkg *packages.Package, f *ast.FuncDecl) {
	fmt.Println(f.Name)
	ast.Inspect(f.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch v := call.Fun.(type) {
		case *ast.SelectorExpr:
			fmt.Println(typeOf(v.X))
			// fmt.Printf("X: %#v\n", v.X)
			// fmt.Printf("Sel: %#v\n", v.Sel.Obj)
			// fmt.Printf("\t%s.%s -> %s.%s\n", pkg, f.Name, v.)
		case *ast.Ident:
			pkgpath := "builtin"
			if v.Obj != nil {
				pkgpath = pkg.String()
			}
			fmt.Printf("\t%s.%s -> %s.%s\n", pkg, f.Name, pkgpath, v.Name)
		default:
			log.Printf("ignoring type call expression with type %T", v)
		}
		return true
	})
}

func typeOf(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		if e.Obj == nil {
			return e.Name
		}
		return fmt.Sprintf("%#v", e.Obj.Decl)
	}
	return fmt.Sprintf("%#v", expr)
}
