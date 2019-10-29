# Code as Graphs

If you think about it, a piece of code can be parsed into a syntax tree.
Now think about relations like "uses", "defined by", etc. Those relationships
connect elements of the syntax tree in new ways eventually creating cycles.

What do you have? A graph!

In this repository I want to create a parser that stores the result of its
parsing into a Graph Database ([Dgraph](https://dgraph.io).

This is an experiment for a talk, so it will probably not be maintained for long.
