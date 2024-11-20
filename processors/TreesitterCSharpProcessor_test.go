package processors

import (
	"context"
	"fmt"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
)

func TestSomethingCSharp(t *testing.T) {

	sourceCode := `
using System;
using MyNamespace.Utilities;

namespace MyNamespace
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.WriteLine("Hello, World!");
        }
    }
}
`

	// Create a new parser and set the language to C#
	parser := sitter.NewParser()
	parser.SetLanguage(csharp.GetLanguage())

	// Parse the source code
	tree, _ := parser.ParseCtx(context.TODO(), nil, []byte(sourceCode))

	// Get the root node of the syntax tree
	rootNode := tree.RootNode()

	// Print the syntax tree
	fmt.Println(rootNode.String())
}

func test_buildTypeToFileMap(fileTrees map[string]*sitter.Tree) map[string]string {
	typeToFile := make(map[string]string)

	for filePath, tree := range fileTrees {
		rootNode := tree.RootNode()
		sourceCode, _ := os.ReadFile(filePath)
		test_collectTypeDeclarations(rootNode, sourceCode, typeToFile, filePath)
	}
	return typeToFile
}

func test_collectTypeDeclarations(node *sitter.Node, sourceCode []byte, typeToFile map[string]string, filePath string) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "class_declaration", "struct_declaration", "interface_declaration", "enum_declaration":
		// Get the name of the type
		identifierNode := node.ChildByFieldName("name")
		if identifierNode != nil {
			typeName := identifierNode.Content(sourceCode)
			typeToFile[typeName] = filePath
		}
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		test_collectTypeDeclarations(child, sourceCode, typeToFile, filePath)
	}
}

func test_analyzeDependencies(fileTrees map[string]*sitter.Tree) map[string]map[string]struct{} {
	dependencies := make(map[string]map[string]struct{})

	for filePath, tree := range fileTrees {
		rootNode := tree.RootNode()
		deps := make(map[string]struct{})
		sourceCode, _ := ioutil.ReadFile(filePath)

		test_walkAndCollectDependencies(rootNode, sourceCode, deps)

		dependencies[filePath] = deps
	}
	return dependencies
}

func test_walkAndCollectDependencies(node *sitter.Node, sourceCode []byte, deps map[string]struct{}) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "qualified_name", "identifier":
		// Possible type or namespace reference
		name := node.Content(sourceCode)
		deps[name] = struct{}{}
	case "using_directive":
		// Using directives (namespaces)
		child := node.ChildByFieldName("name")
		if child != nil {
			name := child.Content(sourceCode)
			deps[name] = struct{}{}
		}
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		test_walkAndCollectDependencies(child, sourceCode, deps)
	}
}

func test_resolveDependenciesToFiles(dependencies map[string]map[string]struct{}, typeToFile map[string]string) map[string]map[string]struct{} {
	fileDependencies := make(map[string]map[string]struct{})

	for file, deps := range dependencies {
		fileDeps := make(map[string]struct{})
		for dep := range deps {
			if depFile, ok := typeToFile[dep]; ok && depFile != file {
				fileDeps[depFile] = struct{}{}
			}
		}
		fileDependencies[file] = fileDeps
	}
	return fileDependencies
}

func test_generateDotFile(fileDependencies map[string]map[string]struct{}, outputPath string) error {
	f, err := os.Create(path.Join(os.TempDir(), outputPath))
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("digraph G {\n")

	for file, deps := range fileDependencies {
		for depFile := range deps {
			fmt.Fprintf(f, "  \"%s\" -> \"%s\";\n", filepath.Base(file), filepath.Base(depFile))
		}
	}

	f.WriteString("}\n")

	fmt.Printf("File Created: %v \n", f.Name())
	return nil
}

func TestSomethingElseCSharp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "csfiles")
	if err != nil {
		t.Fatal(err)
	}

	csFiles := map[string]string{
		"Program.cs": `using System;

namespace SampleApp
{
    class Program
    {
        static void Main(string[] args)
        {
            var greeter = new Greeter();
            greeter.Greet("World");

            var calculator = new Calculator();
            int sum = calculator.Add(5, 7);
            Console.WriteLine($"Sum: {sum}");

            var dataService = new DataService();
            dataService.SaveData("Sample Data");
        }
    }
}`,
		"Greeter.cs": `using System;

namespace SampleApp
{
    public class Greeter
    {
        public void Greet(string name)
        {
            Console.WriteLine($"Hello, {name}!");
        }
    }
}`,
		"Calculator.cs": `namespace SampleApp
{
    public class Calculator
    {
        public int Add(int a, int b)
        {
            return a + b;
        }
    }
}`,
		"DataService.cs": `using System;

namespace SampleApp
{
    public class DataService
    {
        public void SaveData(string data)
        {
            Console.WriteLine($"Data '{data}' has been saved.");
        }
    }
}`,
	}

	for fileName, content := range csFiles {
		err := os.WriteFile(path.Join(tempDir, fileName), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	fileTrees := make(map[string]*sitter.Tree)
	var mu sync.Mutex
	var wg sync.WaitGroup

	//dir := tempDir
	dir := "/home/parallels/Development/clients/defra/epr/epr-laps-home"
	// Walk through the directory and parse each .cs file
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".cs" {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				sourceCode, err := os.ReadFile(path)
				if err != nil {
					log.Printf("Failed to read file %s: %v\n", path, err)
					return
				}

				parser := sitter.NewParser()
				parser.SetLanguage(csharp.GetLanguage())
				tree, _ := parser.ParseCtx(context.TODO(), nil, sourceCode)

				mu.Lock()
				fileTrees[path] = tree
				mu.Unlock()
			}(path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %q: %v\n", dir, err)
	}

	wg.Wait()

	// Proceed to analyze the syntax trees
	typeToFile := test_buildTypeToFileMap(fileTrees)
	dependencies := test_analyzeDependencies(fileTrees)
	fileDependencies := test_resolveDependenciesToFiles(dependencies, typeToFile)

	// Print the file dependencies
	fmt.Println("File Dependencies:")
	for file, deps := range fileDependencies {
		fmt.Printf("%s depends on:\n", filepath.Base(file))
		for depFile := range deps {
			fmt.Printf("  %s\n", filepath.Base(depFile))
		}
	}

	// Optionally, generate a DOT file for visualization
	err = test_generateDotFile(fileDependencies, "dependencies.dot")
	if err != nil {
		log.Fatalf("Failed to generate DOT file: %v\n", err)
	} else {
		fmt.Println("Dependency graph generated in dependencies.dot")
	}

}
