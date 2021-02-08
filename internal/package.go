package internal

import (
	"encoding/xml"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// PackageMetadata represents the metadata of a package generated by ypkg
type PackageMetadata struct {
	XMLName xml.Name `xml:"PISI"`
	Package Package
}

// RuntimeDependencies represents package dependencies used at runtime
type RuntimeDependencies struct {
	Dependencies []Dependency `xml:"Dependency"`
}

// Package represents one of the eopkg archives generated by ypkg
type Package struct {
	Name                string
	Component           string
	RuntimeDependencies RuntimeDependencies
}

// NewPackage returns a fully instanciated package from eopkg information
func NewPackage(name string) (p *Package, err error) {
	cmd := []string{
		"eopkg",
		"info",
		"--xml",
		name,
	}

	o, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		if err.Error() == "exit status 1" {
			return nil, fmt.Errorf("Package %s does not exists", name)
		}

		return
	}

	metadata := &PackageMetadata{}

	r := strings.NewReader(string(o))
	dec := xml.NewDecoder(r)
	err = dec.Decode(metadata)
	if err != nil {
		return
	}

	return &metadata.Package, nil
}

// DependencyGraph returns the dependency graph of a package
func (p Package) DependencyGraph() (d *DependencyGraph, err error) {
	var wg sync.WaitGroup

	d = &DependencyGraph{}
	stack := Stack{}
	seen := make(map[string]bool)

	stack.Push(p)
	for !stack.IsEmpty() {
		current := stack.Pop()
		dependency := &Dependency{current.Name}

		if _, ok := seen[current.Name]; !ok {
			seen[current.Name] = true
			d.AddNode(dependency)

			for _, childDep := range current.RuntimeDependencies.Dependencies {
				// This is needed to avoid overriding fields in the struct when lock is
				// not released before the loop goes to the next value
				childDep := childDep
				d.AddEdge(dependency, &childDep)
			}
		}

		for _, childDep := range current.RuntimeDependencies.Dependencies {
			childDep := childDep
			wg.Add(1)
			go func(childDep Dependency) {
				defer wg.Done()

				// This is needed to avoid overriding fields in the struct when lock is
				// not released before the loop goes to the next value
				if _, ok := seen[childDep.Name]; !ok {
					var childPackage *Package
					childPackage, err = NewPackage(childDep.Name)
					if err != nil {
						return
					}
					stack.Push(*childPackage)
				}
			}(childDep)
		}

		wg.Wait()
	}

	return
}
