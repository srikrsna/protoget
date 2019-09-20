package main

import (
	"protoget"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(protoget.Analyzer)
}