package hadoop

import "os"
import "io"
import "fmt"
import "bytes"
import "github.com/eiiches/go-bzip2"

var SEQ_MAGIC = []byte("SEQ")

const SYNC_HASH_SIZE = 16

const (
	VERSION_BLOCK_COMPRESS  = 4
	VERSION_CUSTOM_COMPRESS = 5
	VERSION_WITH_METADATA   = 6
)

type SequenceFileReader struct {
	sync   []byte
	reader io.Reader
}

func NewSequenceFileReader(r io.Reader) (*SequenceFileReader, error) {
	var magic [3]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, err
	}
	if bytes.Compare(magic[:], SEQ_MAGIC) != 0 {
		return nil, fmt.Errorf("bad magic")
	}
	var version [1]byte
	if _, err := io.ReadFull(r, version[:]); err != nil {
		return nil, err
	}
	if version[0] > VERSION_WITH_METADATA {
		return nil, fmt.Errorf("unsupported version")
	}

	if version[0] < VERSION_BLOCK_COMPRESS {
		return nil, fmt.Errorf("not implemented")
	} else {
		var keyClassName TextWritable
		var valueClassName TextWritable
		keyClassName.Read(r)
		valueClassName.Read(r)
		fmt.Println("keyClassName =", string(keyClassName.Buf))
		fmt.Println("valueClassName =", string(valueClassName.Buf))
	}

	var compressed = false
	if version[0] > 2 {
		var err error
		compressed, err = ReadBoolean(r)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("compressed =", compressed)

	var blockCompressed = false
	if version[0] >= VERSION_BLOCK_COMPRESS {
		var err error
		blockCompressed, err = ReadBoolean(r)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("blockCompressed =", blockCompressed)

	if compressed {
		if version[0] >= VERSION_CUSTOM_COMPRESS {
			var codecClassName TextWritable
			codecClassName.Read(r)
			fmt.Println("codecClassName =", string(codecClassName.Buf))
		} else {
			return nil, fmt.Errorf("not implemented")
		}
	}

	var metadata map[string]string
	if version[0] >= VERSION_WITH_METADATA {
		size, err := ReadInt(r)
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(size); i++ {
			var key TextWritable
			var value TextWritable
			key.Read(r)
			value.Read(r)
			metadata[string(key.Buf)] = string(value.Buf)
		}
	}

	var sync []byte = nil
	if version[0] > 1 {
		sync = make([]byte, SYNC_HASH_SIZE)
		if _, err := io.ReadFull(r, sync[:]); err != nil {
			return nil, err
		}
		fmt.Println("sync =", sync[:])
	}

	return &SequenceFileReader{
		sync:   sync,
		reader: r,
	}, nil
}

func (self *SequenceFileReader) Read(key Writable, value Writable) error {
	if self.sync != nil {
		ReadInt(self.reader)
		var sync [SYNC_HASH_SIZE]byte
		if _, err := io.ReadFull(self.reader, sync[:]); err != nil {
			return err
		}
		if bytes.Compare(sync[:], self.sync) != 0 {
			return fmt.Errorf("sync check failure")
		}
	}

	numRecords, err := ReadVLong(self.reader)
	if err != nil {
		return err
	}
	fmt.Println("numRecords =", numRecords)

	keyLenBuffer, err := ReadBuffer(self.reader)
	if err != nil {
		return err
	}
	fmt.Println("len(keyLenBuffer) =", len(keyLenBuffer))

	fw, err := os.Create("keylenbuf.bz2")
	fw.Write(keyLenBuffer)
	fw.Close()

	fmt.Println("keylenbuf = [", keyLenBuffer[:18], "...]")
	keyLenReader, err := bzip2.NewReader(bytes.NewReader(keyLenBuffer))
	if err != nil {
		return err
	}
	for i := 0; i < int(numRecords); i++ {
		l, err := ReadVLong(keyLenReader)
		if err != nil {
			return err
		}
		fmt.Println(l)
	}

	keyBuffer, err := ReadBuffer(self.reader)
	if err != nil {
		return err
	}
	fmt.Println("len(keyBuffer) =", len(keyBuffer))
	keyReader, err := bzip2.NewReader(bytes.NewReader(keyBuffer))
	if err != nil {
		return err
	}
	for i := 0; i < int(numRecords); i++ {
		var l LongWritable
		if err := l.Read(keyReader); err != nil {
			return err
		}
		fmt.Println(l)
	}

	f2, err := os.Create("keybuf.bz2")
	f2.Write(keyBuffer)
	f2.Close()

	valueLenBuffer, err := ReadBuffer(self.reader)
	if err != nil {
		return err
	}
	fmt.Println("len(valueLenBuffer) =", len(valueLenBuffer))

	valueBuffer, err := ReadBuffer(self.reader)
	if err != nil {
		return err
	}
	fmt.Println("len(valueBuffer) =", len(valueBuffer))

	fp := self.reader.(*os.File)

	pos, _ := fp.Seek(0, os.SEEK_CUR)
	fmt.Println(pos)

	return io.EOF
}