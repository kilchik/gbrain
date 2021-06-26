package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/kilchik/gbrain/internal/pkg/lib"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var BuildCommit string

var ErrNotFound = fmt.Errorf("not found")

type Photos struct {
	items map[string]image.Image
	grd   sync.Mutex
}

func (p *Photos) Add(item image.Image) string {
	p.grd.Lock()
	defer p.grd.Unlock()
	key := generateKey()
	p.items[key] = item
	return key
}

func (p *Photos) Get(key string) (*image.Image, error) {
	p.grd.Lock()
	defer p.grd.Unlock()
	img, ok := p.items[key]
	if !ok {
		return nil, ErrNotFound
	}
	return &img, nil
}

func generateKey() string {
	return uuid.New().String()
}

func NewPhotos() *Photos {
	return &Photos{
		items: make(map[string]image.Image),
	}
}

func main() {
	lib.SayHay()
	fmt.Printf("BuildCommit: %q\n", BuildCommit)

	debug := flag.Bool("debug", false, "set log level to debug")
	flag.Parse()

	log.SetFormatter(&log.JSONFormatter{})

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	photos := NewPhotos()

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		l := log.WithField("ep", "/upload").WithField("agent", r.UserAgent())
		l.Debugf("start handling upload")

		f, size, err := getFileFromReq(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		typ, err := sniffType(f)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		l.Infof("image type: %q", typ)

		if typ != "image/jpeg" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		img, _, err := image.Decode(f)
		if err != nil {
			l.Errorf("decode image: %v", err)
			http.Error(w, "unable to decode", http.StatusBadRequest)
			return
		}

		key := photos.Add(img)

		l.Infof("size: %d", size)

		fmt.Fprint(w, key)
	})

	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		l := log.WithField("ep", "/download").WithField("agent", r.UserAgent())
		l.Debugf("start handling download")

		key := r.URL.Query().Get("key")
		if key == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		img, err := photos.Get(key)
		if err != nil {
			l.Errorf("get with key %q: %v", key, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jpeg.Encode(w, *img, nil)
	})

	log.Infof("starting server at 7777...")
	http.ListenAndServe("localhost:7777", nil)
}

func sniffType(seeker io.ReadSeeker) (string, error) {
	buff := make([]byte, 512)

	_, err := seeker.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}
	defer seeker.Seek(0, io.SeekStart)

	bytesRead, err := seeker.Read(buff)
	if err != nil && err != io.EOF {
		return "", err
	}

	buff = buff[:bytesRead]

	return http.DetectContentType(buff), nil
}

func getFileFromReq(r *http.Request) (multipart.File, int64, error) {
	err := r.ParseMultipartForm(20 * 1024 * 1024)
	if err != nil {
		return nil, 0, errors.Wrap(err, "parse form")
	}

	f, h, err := r.FormFile("file")
	if err != nil {
		return nil, 0, errors.Wrap(err, "find file in form")
	}

	return f, h.Size, nil
}
