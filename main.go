package main

import (
	"DocuLegal/Models"
	"io"
	"log"
	"net/http"
	"os"
	"encoding/csv"
	"fmt"
	//"io/ioutil"
	"strconv"
	"bytes"
	"unicode/utf8"
	"context"
	"strings"
	//"time"
	//"html/template"

	"github.com/jung-kurt/gofpdf"
	"github.com/google/uuid"

	openai "github.com/sashabaranov/go-openai"
)

func main() {
	 // Register the file server as the handler for all requests
	 dir := "./tmp"

	 // Create a new file server for the directory
	 fileServer := http.FileServer(http.Dir(dir))
 
	 // Register the file server on the /static endpoint
	 http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		 http.StripPrefix("/static", fileServer).ServeHTTP(w, r)
	 })
	http.HandleFunc("/DocuLegal", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Serve the index.html file for GET requests
			http.ServeFile(w, r, "index.html")
		} else if r.Method == http.MethodPost {
			// Handle POST requests for the index page here
			uploadHandler(w, r)
		} else {
			// Return a "Method Not Allowed" error for all other types of requests
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/delete", deleteFilesHandler)

	log.Println("Server listening on port 8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10 MB file size limit
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id := uuid.New()
	// Create the 'uploads' directory if it doesn't exist
	err = os.MkdirAll("./tmp/uploads"+id.String(), os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := r.MultipartForm.File["files[]"]
	var UploadedFiles []string
	for i, file := range files {
		// Copy each uploaded file to the 'uploads' directory
		srcFile, err := file.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer srcFile.Close()
		destFile, err := os.Create(fmt.Sprintf("./tmp/uploads"+id.String()+"/%d_%s", i, file.Filename))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		UploadedFiles = append(UploadedFiles, file.Filename)
		fmt.Printf("File %d uploaded: %s\n", i, file.Filename)
	}
	testPrompt := r.FormValue("text")
	//fmt.Printf("Text entered: %s\n", testPrompt)
	//Process files
	chatGPTPrompt, err := ProcessFiles("./tmp/uploads"+id.String()+"/0_"+UploadedFiles[0], "./tmp/uploads"+id.String()+"/1_"+UploadedFiles[1], testPrompt)
	// Write the PDF document to the HTTP response body
	/*data, err := ioutil.ReadFile("template.pdf")
    if err != nil { 
		fmt.Fprint(w, err) 
	}*/
	if err != nil { 
		fmt.Fprint(w, err) 
	}
    //http.ServeContent(w, r, "template.pdf", time.Now(),   bytes.NewReader(data))
	html := `
		<html>
			<head>
  				<meta charset="UTF-8">
  				<title>DocuLegal</title>
			</head>
			<body>
				<h1>Voici le récapitulatif de votre prompt :</h1>
				<p>%s</p>
				<br>
				<p>Voici le lien pour accéder à votre document : <a href="https://delaborde.org/static/template.pdf" target="_blank">Mon document</a>
			</body>
		</html>
	`
	html = fmt.Sprintf(html, chatGPTPrompt)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)

	/*_, err = w.Write(chatGPTTemplate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
		//pdfGeneration(template, clientsDataFilePath)

	}*/
	// Retrieve and print out the value of the text field
	//text := r.FormValue("text")
	//fmt.Printf("Text entered: %s\n", text)

	//fmt.Fprint(w, "Cliquez ici pour accéder à votre document : https://localhost:8080/")
}

func ProcessFiles(excelFilePath string, wordFilePath string, chatGPTPrompt string) (string, error) {
	log.Println("Launched processing..")
	// docx to text
	initialTemplateContent := models.ExtractTextFromWordDocument(wordFilePath)
	//fmt.Println(initialTemplateContent)
	/*err := ioutil.WriteFile("output.txt", []byte(initialTemplateContent), 0644)
	if err != nil {
		panic(err)
	}*/
	// xlsx header to text + first line
	headerList, firstLine := models.ExtractExcelHeader(excelFilePath)
	//fmt.Println(headerList, firstLine)

	//Add header to chatGPTPrompt
	chatGPTPrompt = strings.Replace(chatGPTPrompt, "__header__", strings.Join(headerList, ";"), -1)
	//Add first excel line to chatGPTPrompt
	chatGPTPrompt = strings.Replace(chatGPTPrompt, "__excelFirstRow__", strings.Join(firstLine, ";"), -1)

	//chatgpt prompt assembly
	template := ChatGPT(chatGPTPrompt, initialTemplateContent /*, headerList, firstRowList*/)
	//fmt.Println(template)
	/*err = ioutil.WriteFile("outputChatGPT.txt", []byte(template), 0644)
	if err != nil {
		panic(err)
	}*/

	// Convert the text to a []rune
	runeSlice := []rune(template)

	// Create a new bytes.Buffer
	buffer := bytes.Buffer{}

	// Iterate over the []rune and convert to Windows-1252
	for _, r := range runeSlice {
		if utf8.ValidRune(r) {
			buffer.WriteRune(r)
		} else {
			buffer.WriteRune(utf8.RuneError)
		}
	}

	// Convert the bytes.Buffer to []byte
	byteSlice := buffer.Bytes()

	// Convert the []byte to a string in Windows-1252 encoding
	str := string(windows1252EncodedBytes(byteSlice))
	// Create a new PDF
	pdf := gofpdf.New("P", "mm", "A4", "")

	// Add a new page to the PDF
	pdf.AddPage()

	pdf.SetFont("Times", "", 12)

	// Write the text to the PDF
	pdf.MultiCell(0, 10, str, "", "", false)

	// Output to a file
	err := pdf.OutputFileAndClose("./tmp/template.pdf")
	if err != nil {
		log.Println(err)
		return chatGPTPrompt, err 
	}

	//Delete all files
	deleteFiles()
	//PDF output
	/*
	return pdf.CreateTemplate(func(tpl *gofpdf.Tpl) {
		//tpl.Image(example.ImageFile("logo.png"), 6, 6, 30, 0, false, "", 0, "")
		tpl.SetFont("Arial", "", 14)
		//tpl.Text(40, 20, "Template says hello")
		//tpl.SetDrawColor(0, 100, 200)
		//tpl.SetLineWidth(2.5)
		//tpl.Line(95, 12, 105, 22)
	}).Bytes()*/
	return chatGPTPrompt, nil
}

func windows1252EncodedBytes(b []byte) []byte {
	var windows1252Encoded []byte

	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		if r == utf8.RuneError {
			windows1252Encoded = append(windows1252Encoded, byte(r))
		} else {
			windows1252Encoded = append(windows1252Encoded, byte(r))
		}
	}
	return windows1252Encoded
}

func ChatGPT(prompt string, paragraph string /*, headerList string, firstRowList string*/) string {
	client := openai.NewClient("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")
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt + paragraph,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "No answer from chatGPT"
	}

	//fmt.Println(resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content
}

func pdfGeneration(template string, clientsDataFilePath string) {
	// Ouverture du fichier CSV
	f, err := os.Open(clientsDataFilePath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	// Parsage du fichier CSV
	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Itérer sur chaque ligne du fichier CSV
	for i, row := range rows {
		if i == 0 { // Ignorer la première ligne
			continue
		}
		siren, _ := strconv.Atoi(row[0])
		montant, _ := strconv.ParseFloat(row[1], 64)
		filename := fmt.Sprintf("%d.pdf", siren) // Nom du fichier PDF à générer

		// Créer un nouveau PDF et y ajouter le texte
		pdf := gofpdf.New(gofpdf.OrientationPortrait, gofpdf.UnitPoint, gofpdf.PageSizeLetter, "")
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		text := fmt.Sprintf("Bonjour %d, par rapport à votre deal de %v.", siren, montant)
		pdf.Cell(0, 60, text)
		pdf.OutputFileAndClose(filename)
	}

	fmt.Println("PDF généré avec succès !")
}

func deleteFilesHandler(w http.ResponseWriter, r *http.Request) {
	err := os.RemoveAll("/tmp/uploads")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Files deleted successfully")
}

func deleteFiles() {
	err := os.RemoveAll("/tmp/uploads")
	if err != nil {
		log.Fatal(err)
	}
}
func deleteFilesInternal() error {
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/delete", nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete files: %s", res.Status)
	}

	return nil
}

//chatgpt to template
//fill template
//Loop for template
