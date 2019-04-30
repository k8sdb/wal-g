package internal

import (
	"github.com/DataDog/zstd"
	"github.com/pkg/errors"
	"github.com/wal-g/wal-g/utility"
	"io"
)

type ZstdDecompressor struct{}

func (decompressor ZstdDecompressor) Decompress(dst io.Writer, src io.Reader) error {
	zstdReader := zstd.NewReader(NewUntilEofReader(src))
	_, err := utility.FastCopy(dst, zstdReader)
	if err != nil {
		return errors.Wrap(err, "DecompressZstd: zstd write failed")
	}
	err = zstdReader.Close()
	return errors.Wrap(err, "DecompressZstd: zstd reader close failed")
}

func (decompressor ZstdDecompressor) FileExtension() string {
	return ZstdFileExtension
}
