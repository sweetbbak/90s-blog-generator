package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/russross/blackfriday"
)

var (
	files []string
	tail  []string
	dir   string
)

var usage = `gensite
USAGE:
	gensite -f <file.md>
	gensite -f <file.md> -f <file.md>
	gensite -d <directory>
	
OPTIONS:
	-h | --help       print help
	-f | --file       add files to convert to HTML
	-d | --directory  path to directory with md files
	`

func init() {

	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Println(usage)
		os.Exit(0)
	}

	for x := range os.Args {
		arg := os.Args[x]
		switch arg {
		case "--file", "-f":
			files = append(files, os.Args[x+1])
			x = x + 1
		case "--directory", "-d":
			dir = os.Args[x+1]
		default:
			tail = append(tail, os.Args[x])
		}
	}
}

func main() {
	if len(files) < 1 && dir == "" {
		fmt.Println(usage)
		os.Exit(0)
	}

	for x := range files {
		fname := files[x]
		bname := strings.SplitN(fname, ".", 2)[0]
		ext := strings.SplitN(fname, ".", 2)[1]

		if strings.ToLower(ext) == "md" {
			convert(files[x], bname)
			fmt.Printf("Created file: \x1b[35m%v\x1b[0m\n", bname+".html")
		}
	}

	if dir != "" {
		readDir(dir)
	}
}

func readDir(dir string) {
	dfiles, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println(err)
	}

	var counter int16 = 0
	for _, fi := range dfiles {
		path := filepath.Join(dir, fi.Name())
		stat, err := os.Stat(path)
		if err != nil {
			fmt.Println(err)
		}

		if !stat.IsDir() && strings.SplitN(stat.Name(), ".", 2)[1] == "md" {
			counter++
			convert(fi.Name(), strings.SplitN(fi.Name(), ".", 2)[0])
		}

	}
	fmt.Printf("Processed %d files\n", counter)
}

func convert(inputFile string, baseName string) {
	// load markdown file
	mdFile, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatal(err)
	}
	// convert markdown to html
	htmlSrc := blackfriday.MarkdownCommon(mdFile)
	// replace code-parts with syntax-highlighted parts
	replaced, err := replaceCodeParts(htmlSrc)
	if err != nil {
		log.Fatal(err)
	}
	// read template
	t, err := template.ParseFiles("./template.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	// write css
	hlbuf := bytes.Buffer{}
	hlw := bufio.NewWriter(&hlbuf)
	formatter := html.New(html.WithClasses(true))
	if err := formatter.WriteCSS(hlw, styles.MonokaiLight); err != nil {
		log.Fatal(err)
	}
	hlw.Flush()

	file, err := os.Create(baseName + ".html")
	if err != nil {
		fmt.Println(err)
	}

	// write html output
	if err := t.Execute(file, struct {
		Content template.HTML
		Style   template.CSS
	}{
		Content: template.HTML(replaced),
		Style:   template.CSS(hlbuf.String()),
	}); err != nil {
		log.Fatal(err)
	}
}

func replaceCodeParts(mdFile []byte) (string, error) {
	byteReader := bytes.NewReader(mdFile)
	doc, err := goquery.NewDocumentFromReader(byteReader)
	if err != nil {
		return "", err
	}
	// find code-parts via selector and replace them with highlighted versions
	var hlErr error
	doc.Find("code[class*=\"language-\"]").Each(func(i int, s *goquery.Selection) {
		if hlErr != nil {
			return
		}
		class, _ := s.Attr("class")
		lang := strings.TrimPrefix(class, "language-")
		oldCode := s.Text()
		lexer := lexers.Get(lang)
		formatter := html.New(html.WithClasses(true))
		// formatter := html.New()
		iterator, err := lexer.Tokenise(nil, string(oldCode))
		if err != nil {
			hlErr = err
			return
		}
		b := bytes.Buffer{}
		buf := bufio.NewWriter(&b)
		if err := formatter.Format(buf, styles.Monokai, iterator); err != nil {
			hlErr = err
			return
		}
		if err := buf.Flush(); err != nil {
			hlErr = err
			return
		}
		s.SetHtml(b.String())
	})
	if hlErr != nil {
		return "", hlErr
	}
	new, err := doc.Html()
	if err != nil {
		return "", err
	}
	// replace unnecessarily added html tags
	return new, nil
}
