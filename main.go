//go:build darwin || windows

package main

import (
	"flag"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/adrium/goheif"
)

func main() {
	src := ""
	target := ""
	flag.StringVar(&src, "s", "./", "source directory ")
	flag.StringVar(&target, "t", "./jpgs/", "target directory ")
	flag.Parse()
	if !exist(target) {
		fmt.Printf("target directory not exist, creating...\n")
		err := os.Mkdir(target, os.ModePerm)
		if err != nil {
			fmt.Printf("mkdir failed![%v]\n", err)
		} else {
			fmt.Printf("mkdir success!\n")
		}
	}
	files, err := ioutil.ReadDir(src)
	if err != nil {
		fmt.Printf("Failed to Read dir: %v\n", err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if isCorrectHeic(src + "/" + f.Name()) {
			err := convert(src+"/"+f.Name(), target+"/"+f.Name()+".jpg")
			if err != nil {
				fmt.Printf("Failed to convert: %v\n", err)
			} else {
				fmt.Printf("%s convert success\n", src+"/"+f.Name())
			}
		}
	}
	fmt.Printf("All picture convert success in %s.", src)
}

func isCorrectHeic(fin string) bool {
	if len(fin) < 6 {
		return false
	}
	if strings.ToLower(fin[len(fin)-4:]) != "heic" {
		return false
	}
	mime := getFileContentType(fin)
	// fmt.Printf("%s type is %s\n", fin, mime)
	return mime == "image/heic" || mime == "image/heif" || mime == "application/octet-stream"
}

func getFileContentType(fin string) string {
	// Open File
	out, err := os.Open(fin)
	if err != nil {
		return fmt.Sprintf("Open failed![%v]\n", err)
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			fmt.Printf("Close failed![%v]\n", err)
		}
	}(out)

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err = out.Read(buffer)
	if err != nil {
		return fmt.Sprintf("Read failed![%v]\n", err)
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	return contentType
}

func convert(input, output string) error {
	fileInput, err := os.Open(input)
	if err != nil {
		return err
	}
	defer func(fileInput *os.File) {
		err := fileInput.Close()
		if err != nil {
			fmt.Printf("Close failed![%v]\n", err)
		}
	}(fileInput)

	exif, err := goheif.ExtractExif(fileInput)
	if err != nil {
		return err
	}

	img, err := goheif.Decode(fileInput)
	if err != nil {
		return err
	}
	fileOutput, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer func(fileOutput *os.File) {
		err := fileOutput.Close()
		if err != nil {
			fmt.Printf("Close failed![%v]\n", err)
		}
	}(fileOutput)

	w, _ := newWriterExif(fileOutput, exif)
	err = jpeg.Encode(w, img, nil)
	if err != nil {
		return err
	}

	return nil
}

type writerSkipper struct {
	w           io.Writer
	bytesToSkip int
}

func (w *writerSkipper) Write(data []byte) (int, error) {
	if w.bytesToSkip <= 0 {
		return w.w.Write(data)
	}

	if dataLen := len(data); dataLen < w.bytesToSkip {
		w.bytesToSkip -= dataLen
		return dataLen, nil
	}

	if n, err := w.w.Write(data[w.bytesToSkip:]); err == nil {
		n += w.bytesToSkip
		w.bytesToSkip = 0
		return n, nil
	} else {
		return n, err
	}
}

func newWriterExif(w io.Writer, exif []byte) (io.Writer, error) {
	writer := &writerSkipper{w, 2}
	soi := []byte{0xff, 0xd8}
	if _, err := w.Write(soi); err != nil {
		return nil, err
	}

	if exif != nil {
		app1Marker := 0xe1
		markerlen := 2 + len(exif)
		marker := []byte{0xff, uint8(app1Marker), uint8(markerlen >> 8), uint8(markerlen & 0xff)}
		if _, err := w.Write(marker); err != nil {
			return nil, err
		}

		if _, err := w.Write(exif); err != nil {
			return nil, err
		}
	}

	return writer, nil
}

func exist(dir string) bool {
	_, err := os.Stat(dir)
	if err != nil {
		return os.IsExist(err)
	}
	return true
}
