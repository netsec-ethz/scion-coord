package main

import (
	"flag"
	"log"
	"sort"

	"github.com/netsec-ethz/rains/internal/pkg/object"
	"github.com/netsec-ethz/rains/internal/pkg/section"
	"github.com/netsec-ethz/rains/internal/pkg/zonefile"
)

func main() {

	// flags
	file := flag.String("zf", "zonefile.txt", "Path to zonefile")
	name := flag.String("n", "", "name which is added to zonefile")
	value := flag.String("v", "", "value being added to zonefile")

	flag.Parse()

	if *name == "" || *value == "" {
		log.Fatal("Error: name and value cannot be empty")
	}

	zone, err := zonefile.IO{}.LoadZonefile(*file)
	if err != nil {
		log.Fatal(err)
	}
	sections := []section.Section{}
	for _, e := range zone {
		sections = append(sections, e)
	}
	if len(sections) < 1 {
		log.Fatal("No zone found")
	}
	z, ok := sections[0].(*section.Zone)
	if !ok {
		log.Fatal("No zone found")
	}
	assertions := []*section.Assertion{}
	for _, e := range z.Content {
		// if name already present discard it
		if e.SubjectName == *name {
			continue
		}
		assertions = append(assertions, e)
	}

	// create new scionip4 assertion for name with given value
	obj := object.Object{Type: object.OTScionAddr4, Value: *value}
	assertion := section.Assertion{SubjectName: *name, Content: []object.Object{obj}}

	// add back the other assertions plus the newly created one
	z.Content = assertions
	z.Content = append(z.Content, &assertion)
	// ensure sorted ordere
	sort.Slice(z.Content, func(i, j int) bool {
		return z.Content[i].SubjectName < z.Content[j].SubjectName
	})
	zonefile.IO{}.EncodeAndStore(*file, sections)
}
