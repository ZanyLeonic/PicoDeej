package deej

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/ncruces/zenity"
	"github.com/zanyleonic/picodeej/pkg/deej/util"
	"go.uber.org/zap"
)

type ImageUploadState struct {
	Lock           sync.Mutex
	LoadedFile     []byte
	TotalBytesSent int
	Dialog         *zenity.ProgressDialog
}

type MultiUploadState struct {
	Lock           sync.Mutex
	LoadedFiles    [][]byte
	TotalBytesSent []int
	Dialog         *zenity.ProgressDialog
	CurrentItem    int
}


func (sio *SerialIO) StartImageUpload(logger *zap.SugaredLogger, path string) error {
	sio.currentUpload = &ImageUploadState{}

	sio.currentUpload.Lock.Lock()
	defer sio.currentUpload.Lock.Unlock()

	logger.Debugw("Attempting to open image", "path", path)
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	logger.Debug("Attempting to read bytes from file")
	dat, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	logger.Debugw("Creating progress dialog")
	dlg, err := zenity.Progress(zenity.Title("Uploading Image"), zenity.NoCancel())
	if err != nil {
		return err
	}

	err = dlg.Text(fmt.Sprintf("0/%d", len(dat)))
	if err != nil {
		return err
	}

	sio.currentUpload.LoadedFile = dat
	sio.currentUpload.TotalBytesSent = 0
	sio.currentUpload.Dialog = &dlg

	sio.messageQueue <- []byte(fmt.Sprintf("sendstaticimg %d", len(dat)))

	return nil
}

func (sio *SerialIO) StartAnimatedUpload(logger *zap.SugaredLogger, file string) error {
	panic("unimplemented")
}


func (sio *SerialIO) transferBlock(logger *zap.SugaredLogger, line string) error {
	cu := sio.currentUpload
	if cu == nil {
		return errors.New("current upload object is empty")
	}

	logger.Debugw("Parsing line ", "line", line)

	cu.Lock.Lock()
	defer cu.Lock.Unlock()

	dlg := *cu.Dialog

	if cu.TotalBytesSent >= len(cu.LoadedFile) {
		err := dlg.Complete()

		if err != nil {
			logger.Warnw("Issues cleaning up progress dialog", "error", err)
		}
		logger.Info("Upload completed")

		return nil
	}

	end := util.MinInt(cu.TotalBytesSent+UploadBlock, len(cu.LoadedFile))
	sio.messageQueue <- cu.LoadedFile[cu.TotalBytesSent:end]
	cu.TotalBytesSent = end

	err := dlg.Value(int((float64(cu.TotalBytesSent) / float64(len(cu.LoadedFile))) * 100))
	err = dlg.Text(fmt.Sprintf("Uploaded %d/%d bytes", cu.TotalBytesSent, len(cu.LoadedFile)))

	if err != nil {
		logger.Warnw("Issues updating progress dialog", "error", err)
	}

	logger.Debugw("Sent bytes to controller", "sent", cu.TotalBytesSent, "total", len(cu.LoadedFile))

	return nil
}

func (sio *SerialIO) transferAnimated(logger *zap.SugaredLogger, line string) error {
	panic("unimplemented")
}

func (sio *SerialIO) handleTransferError(logger *zap.SugaredLogger, line string) {
	cus := sio.currentUpload

	cleaned := strings.TrimSuffix(line, "\r\n")
	split := strings.Split(cleaned, ",")

	reason := "Unknown"
	if len(split) > 1 {
		reason = split[1]
	} else {
		logger.Warnw("Controller did not give a reason for failure", "line", split)
	}

	logger.Errorw("Failed to transfer image to microcontroller", "reason", reason)

	cus.Lock.Lock()
	defer cus.Lock.Unlock()

	cus.LoadedFile = []byte{}
	cus.TotalBytesSent = 0

	dlg := *cus.Dialog
	dlg.Text(fmt.Sprintf("Transfer FAILED! Reason: %s", reason))
	dlg.Complete()
}
