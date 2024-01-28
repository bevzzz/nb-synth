package synth_test

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/bevzzz/nb"
	synth "github.com/bevzzz/nb-synth"
	"github.com/bevzzz/nb/pkg/test"
	"github.com/bevzzz/nb/schema"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

// update allows updating golden files via `go test -update`.
var update = flag.Bool("update", false, "update .golden files in testdata/")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestHighlighting(t *testing.T) {
	for _, tt := range []struct {
		name         string
		code         schema.Cell
		highlighting nb.Extension
		golden       string
	}{
		{
			name: "default hightlighting (language)",
			code: &test.CodeCell{
				Cell: test.Cell{
					CellType: schema.Code,
					Mime:     "application/x+python",
					Source:   []byte(snippet),
				},
				Lang: "python",
			},
			highlighting: synth.Highlighting,
			golden:       "code_default",
		},
		{
			name: "default highlighting (mime-type)",
			code: &test.CodeCell{
				Cell: test.Cell{
					CellType: schema.Code,
					Mime:     "application/x-python",
					Source:   []byte(snippet),
				},
			},
			highlighting: synth.Highlighting,
			golden:       "code_default",
		},
		{
			name: "guessing the language",
			code: &test.CodeCell{
				Cell: test.Cell{
					CellType: schema.Code,
					Mime:     "application/x+python",
					Source:   []byte(snippet),
				},
			},
			highlighting: synth.NewHighlighting(
				synth.WithGuessLanguage(true),
			),
			golden: "code_guess",
		},
		{
			name: "with chroma format options",
			code: &test.CodeCell{
				Cell: test.Cell{
					CellType: schema.Code,
					Mime:     "application/x+python",
					Source:   []byte(snippet),
				},
				Lang: "python",
			},
			highlighting: synth.NewHighlighting(
				synth.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.WithLineNumbers(true),
				),
			),
			golden: "code_format_options",
		},
		{
			name: "monokai style",
			code: &test.CodeCell{
				Cell: test.Cell{
					CellType: schema.Code,
					Mime:     "application/x+python",
					Source:   []byte(snippet),
				},
				Lang: "python",
			},
			highlighting: synth.NewHighlighting(
				synth.WithStyle("monokai"),
			),
			golden: "code_monokai",
		},
		{
			name:         "display_data json",
			code:         test.DisplayData(jsonOutput, "application/json"),
			highlighting: synth.Highlighting,
			golden:       "json",
		},
		{
			name:         "execute_result json",
			code:         test.ExecuteResult(jsonOutput, "application/json", 1),
			highlighting: synth.Highlighting,
			golden:       "json",
		},
		{
			name:         "display_data text xml",
			code:         test.DisplayData(xmlOutput, "text/xml"),
			highlighting: synth.Highlighting,
			golden:       "xml_output",
		},
		{
			name:         "display_data application xml",
			code:         test.DisplayData(xmlOutput, "application/xml"),
			highlighting: synth.Highlighting,
			golden:       "xml_output",
		},
		{
			name:         "execute_result application/*+xml",
			code:         test.ExecuteResult(xmlOutput, "application/atom+xml", 1),
			highlighting: synth.Highlighting,
			golden:       "xml_output",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var got bytes.Buffer
			c := nb.New(
				nb.WithExtensions(tt.highlighting),
				nb.WithRenderOptions(test.NoWrapper),
			)
			r := c.Renderer()

			// Act
			err := r.Render(&got, test.Notebook(tt.code))
			require.NoError(t, err)

			// Assert
			cmpGolden(t, "testdata/"+tt.golden+".golden", got.Bytes(), *update)
		})
	}
}

const (
	// Python code snippet
	snippet = `import math

def greet(name):
	print(f"Hello, {name}!")

numbers = [1, 2, 3, 4, 5]
squared_numbers = [math.pow(num, 2) for num in numbers]
print(squared_numbers)

class Animal:
	def __init__(self, species, sound):
		self.species = species
		self.sound = sound

cat = Animal("Cat", "Meow")
print(f"A {cat.species} says {cat.sound}")

# Simple Fibonacci sequence generator
def fibonacci(n):
	fib_sequence = [0, 1]
	while len(fib_sequence) < n:
		fib_sequence.append(fib_sequence[-1] + fib_sequence[-2])
	print(fib_sequence)

fibonacci(10)
`
	// Raw JSON output
	jsonOutput = `{"name": "John", "age": 30, "car": null}`

	// Raw XML output
	xmlOutput = `<?xml version="1.0" encoding="UTF-8"?>
<note>
	<to>Tove</to>
	<from>Jani</from>
	<heading>Reminder</heading>
	<body>Don't forget me this weekend!</body>
</note>`
)

// cmpGolden compares the result of the test run with a golden file.
// If the contents don't match and upd == true, it will update the golden file
// with the current value instead of failing the test.
func cmpGolden(tb testing.TB, goldenFile string, got []byte, upd bool) {
	gf, err := os.OpenFile(goldenFile, os.O_RDWR, 0644)
	require.NoError(tb, err)
	defer gf.Close()

	want, err := io.ReadAll(gf)
	require.NoError(tb, err)

	dotnew := gf.Name() + ".new"
	del := func() {
		files, _ := filepath.Glob("testdata/*.golden.new")
		for i := range files {
			if err := os.Remove(files[i]); err != nil {
				tb.Log(err)
				continue
			}
			log.Printf("deleted previous %s file", dotnew)
		}
	}

	if bytes.Equal(want, got) {
		del()
		return
	}

	if upd {
		err = gf.Truncate(0)
		require.NoError(tb, err)

		gf.Seek(0, 0)
		_, err := gf.Write(got)
		require.NoError(tb, err)

		log.Printf("updated %s", goldenFile)
		del()
		return
	}

	tb.Errorf("mismatched output (+want) (-got):\n%s", cmp.Diff(string(want), string(got)))

	if err := os.WriteFile(dotnew, got, 0644); err == nil {
		tb.Logf("saved to %s (the file will be deleted on the next `-update` or successful test run)", dotnew)
	} else {
		tb.Logf("failed to save %s: %v", dotnew, err)
	}
}
