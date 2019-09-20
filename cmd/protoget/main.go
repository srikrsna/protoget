package main

import (
	"github.com/srikrsna/protoget"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(protoget.Analyzer)
}