package sarama

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"sync"

	snappy "github.com/eapache/go-xerial-snappy"
	"github.com/pierrec/lz4/v4"
)

var (
	lz4ReaderPool = sync.Pool{
		New: func() interface{} {
			return lz4.NewReader(nil)
		},
	}

	gzipReaderPool sync.Pool

	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	bytesPool = sync.Pool{
		New: func() interface{} {
			res := make([]byte, 0, 4096)
			return &res
		},
	}
)

func decompress(cc CompressionCodec, data []byte) ([]byte, error) {
	switch cc {
	case CompressionNone:
		return data, nil
	case CompressionGZIP:
		var err error
		reader, ok := gzipReaderPool.Get().(*gzip.Reader)
		if !ok {
			reader, err = gzip.NewReader(bytes.NewReader(data))
		} else {
			err = reader.Reset(bytes.NewReader(data))
		}

		if err != nil {
			return nil, err
		}

		buffer := bufferPool.Get().(*bytes.Buffer)
		_, err = buffer.ReadFrom(reader)
		// copy the buffer to a new slice with the correct length
		// reuse gzipReader and buffer
		gzipReaderPool.Put(reader)
		res := make([]byte, buffer.Len())
		copy(res, buffer.Bytes())
		buffer.Reset()
		bufferPool.Put(buffer)

		return res, err
	case CompressionSnappy:
		return snappy.Decode(data)
	case CompressionLZ4:
		reader, ok := lz4ReaderPool.Get().(*lz4.Reader)
		if !ok {
			reader = lz4.NewReader(bytes.NewReader(data))
		} else {
			reader.Reset(bytes.NewReader(data))
		}
		buffer := bufferPool.Get().(*bytes.Buffer)
		_, err := buffer.ReadFrom(reader)
		// copy the buffer to a new slice with the correct length
		// reuse lz4Reader and buffer
		lz4ReaderPool.Put(reader)
		res := make([]byte, buffer.Len())
		copy(res, buffer.Bytes())
		buffer.Reset()
		bufferPool.Put(buffer)

		return res, err
	case CompressionZSTD:
		buffer := *bytesPool.Get().(*[]byte)
		var err error
		buffer, err = zstdDecompress(ZstdDecoderParams{}, buffer, data)
		// copy the buffer to a new slice with the correct length and reuse buffer
		res := make([]byte, len(buffer))
		copy(res, buffer)
		buffer = buffer[:0]
		bytesPool.Put(&buffer)

		return res, err
	default:
		return nil, PacketDecodingError{fmt.Sprintf("invalid compression specified (%d)", cc)}
	}
}
