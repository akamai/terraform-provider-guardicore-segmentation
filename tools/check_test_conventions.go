package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := filepath.Clean(filepath.Join("internal", "provider"))
	if _, err := os.Stat(root); err != nil {
		alt := filepath.Clean(filepath.Join("..", "internal", "provider"))
		if _, altErr := os.Stat(alt); altErr == nil {
			root = alt
		}
	}

	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") && !strings.HasSuffix(path, "_acc_test.go") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"acctest"+string(filepath.Separator)) {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)

		isAccFile := strings.HasSuffix(path, "_acc_test.go")
		hasTestAcc := strings.Contains(text, "func TestAcc")
		hasAccImports := strings.Contains(text, "internal/provider/acctest") || strings.Contains(text, "terraform-plugin-testing/helper/resource")

		if isAccFile {
			if !hasTestAcc && strings.HasSuffix(path, string(filepath.Separator)+"testmain_acc_test.go") == false {
				violations = append(violations, fmt.Sprintf("%s: _acc_test.go file should contain TestAcc tests", path))
			}
			return nil
		}

		if hasTestAcc {
			violations = append(violations, fmt.Sprintf("%s: TestAcc found outside *_acc_test.go", path))
		}
		if hasAccImports {
			violations = append(violations, fmt.Sprintf("%s: acceptance-only imports found outside *_acc_test.go", path))
		}

		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "check failed: %v\n", err)
		os.Exit(1)
	}

	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "test convention violations:")
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "- %s\n", v)
		}
		os.Exit(1)
	}

	fmt.Println("test conventions check passed")
}
