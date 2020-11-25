package squashfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

type metadata struct {
	raw        uint16
	size       uint16
	compressed bool
}

//MetadataReader is a block reader for metadata. It will automatically read the next block, when it reaches the end of a block.
type MetadataReader struct {
	s          *Reader
	offset     int64
	headers    []*metadata
	data       []byte
	readOffset int
}

//NewMetadataReader creates a new MetadataReader, beginning to read at the given offset. It will automatically cache the first block at the location.
func (s *Reader) NewMetadataReader(offset int64) (*MetadataReader, error) {
	var br MetadataReader
	br.s = s
	br.offset = offset
	err := br.parseMetadata()
	if err != nil {
		return nil, err
	}
	err = br.readNextDataBlock()
	if err != nil {
		return nil, err
	}
	return &br, nil
}

//NewMetadataReaderFromInodeRef creates a new MetadataReader with the offsets set by the given inode reference.
func (s *Reader) NewMetadataReaderFromInodeRef(ref uint64) (*MetadataReader, error) {
	offset, metaOffset := processInodeRef(ref)
	br, err := s.NewMetadataReader(int64(s.super.InodeTableStart + offset))
	if err != nil {
		return nil, err
	}
	_, err = br.Seek(int64(metaOffset), io.SeekStart)
	if err != nil {
		return nil, err
	}
	return br, nil
}

func (br *MetadataReader) parseMetadata() error {
	var raw uint16
	err := binary.Read(io.NewSectionReader(br.s.r, br.offset, 2), binary.LittleEndian, &raw)
	if err != nil {
		return err
	}
	br.offset += 2
	compressed := !(raw&0x8000 == 0x8000)
	size := raw &^ 0x8000
	br.headers = append(br.headers, &metadata{
		raw:        raw,
		size:       size,
		compressed: compressed,
	})
	return nil
}

func (br *MetadataReader) readNextDataBlock() error {
	meta := br.headers[len(br.headers)-1]
	r := io.NewSectionReader(br.s.r, br.offset, int64(meta.size))
	if meta.compressed {
		byts, err := br.s.decompressor.Decompress(r)
		if err != nil {
			return err
		}
		br.offset += int64(meta.size)
		br.data = append(br.data, byts...)
		return nil
	}
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		return err
	}
	br.offset += int64(meta.size)
	br.data = append(br.data, buf.Bytes()...)
	return nil
}

//Read reads bytes into the given byte slice. Returns the amount of data read.
func (br *MetadataReader) Read(p []byte) (int, error) {
	if br.readOffset+len(p) < len(br.data) {
		for i := 0; i < len(p); i++ {
			p[i] = br.data[br.readOffset+i]
		}
		br.readOffset += len(p)
		return len(p), nil
	}
	read := 0
	for read < len(p) {
		err := br.parseMetadata()
		if err != nil {
			br.readOffset += read
			return read, err
		}
		err = br.readNextDataBlock()
		if err != nil {
			br.readOffset += read
			return read, err
		}
		for ; read < len(p); read++ {
			if br.readOffset+read < len(br.data) {
				p[read] = br.data[br.readOffset+read]
			} else {
				break
			}
		}
	}
	br.readOffset += read
	if read != len(p) {
		return read, errors.New("Didn't read enough data")
	}
	return read, nil
}

//Seek will seek to the specified location (if possible). Seeking is relative to the uncompressed data.
//When io.SeekCurrent or io.SeekStart is set, if seeking would put the offset beyond the current cached data, it will try to read the next data blocks to accomodate. On a failure it will seek to the end of the data.
//When io.SeekEnd is set, it wil seek reletive to the currently cached data.
func (br *MetadataReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		br.readOffset += int(offset)
		for {
			if br.readOffset < len(br.data) {
				break
			}
			err := br.parseMetadata()
			if err != nil {
				br.readOffset = len(br.data)
				return int64(br.readOffset), err
			}
			err = br.readNextDataBlock()
			if err != nil {
				br.readOffset = len(br.data)
				return int64(br.readOffset), err
			}
		}
	case io.SeekStart:
		br.readOffset = int(offset)
		for {
			if br.readOffset < len(br.data) {
				break
			}
			err := br.parseMetadata()
			if err != nil {
				br.readOffset = len(br.data)
				return int64(br.readOffset), err
			}
			err = br.readNextDataBlock()
			if err != nil {
				br.readOffset = len(br.data)
				return int64(br.readOffset), err
			}
		}
	case io.SeekEnd:
		br.readOffset = len(br.data) - int(offset)
		if br.readOffset < 0 {
			br.readOffset = 0
			return int64(br.readOffset), errors.New("Trying to seek to a negative value")
		}
	}
	return int64(br.readOffset), nil
}
