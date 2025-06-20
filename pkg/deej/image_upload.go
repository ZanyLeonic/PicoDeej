package deej

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
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
}

type MultiUploadState struct {
	Lock           sync.Mutex
	LoadedFiles    [][]byte
	TotalBytesSent []int
	CurrentItem    int
}

var animatedFramePattern = regexp.MustCompile(`^frame_\d{2}_delay-\d+(\.\d+)?s\.png$`)

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
	sio.transferDialog = &dlg

	sio.messageQueue <- []byte(fmt.Sprintf("sendstaticimg %d\r\n", len(dat)))

	return nil
}

func (sio *SerialIO) StartAnimatedUpload(logger *zap.SugaredLogger, path string) error {
	sio.currentMultiUpload = &MultiUploadState{}

	sio.currentMultiUpload.Lock.Lock()
	defer sio.currentMultiUpload.Lock.Unlock()

	logger.Debugw("Attempting to open image", "path", path)
	zipF, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zipF.Close()

	sio.currentMultiUpload.LoadedFiles = make([][]byte, 0, 120)

	for i, file := range zipF.File {
		cFormat := animatedFramePattern.MatchString(file.Name)

		logger.Debugw(fmt.Sprintf("[%d]: %s", i, file.Name))
		logger.Debugw("Passes regex?", "pass", cFormat)

		if !cFormat {
			sio.currentMultiUpload = &MultiUploadState{}
			return errors.New("animated image set is not in the correct format")
		}

		fileInZip, err := file.Open()
		if err != nil {
			return err
		}

		raw, err := ioutil.ReadAll(fileInZip)
		if err != nil {
			return err
		}

		sio.currentMultiUpload.LoadedFiles = append(sio.currentMultiUpload.LoadedFiles, raw)
	}

	totalFiles := len(sio.currentMultiUpload.LoadedFiles)

	logger.Debugw("Creating progress dialog")
	dlg, err := zenity.Progress(zenity.Title("Uploading Image"), zenity.NoCancel())
	if err != nil {
		return err
	}

	err = dlg.Text(fmt.Sprintf("Uploaded frame 0/%d", totalFiles))
	if err != nil {
		return err
	}

	sio.currentMultiUpload.CurrentItem = 0
	sio.currentMultiUpload.TotalBytesSent = make([]int, totalFiles)
	sio.transferDialog = &dlg

	sio.messageQueue <- []byte(fmt.Sprintf("sendanimatedimg %d\r\n", totalFiles))

	return nil
}

func (sio *SerialIO) transferBlock(logger *zap.SugaredLogger, line string) error {
	cu := sio.currentUpload
	if cu == nil {
		return errors.New("current upload object is empty")
	}

	logger.Debugw("Parsing line ", "line", line)

	cu.Lock.Lock()
	defer cu.Lock.Unlock()

	dlg := *sio.transferDialog

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
	mcu := sio.currentMultiUpload
	if mcu == nil {
		return errors.New("multi upload object is empty")
	}

	logger.Debugw("Parsing line ", "line", line)
	lastResp := strings.TrimSuffix(line, "\r\n")

	mcu.Lock.Lock()
	defer mcu.Lock.Unlock()

	cItem := mcu.CurrentItem
	total := len(mcu.LoadedFiles) - 1
	dlg := *sio.transferDialog

	if cItem > total {
		err := dlg.Complete()

		if err != nil {
			logger.Warnw("Issues cleaning up progress dialog", "error", err)
		}

		logger.Infow("Upload completed", "total", total)

		return nil
	}

	if mcu.TotalBytesSent[cItem] >= len(mcu.LoadedFiles[cItem]) {
		logger.Infow("Finished uploading frame", "file", mcu.CurrentItem, "totalFiles", total)

		sio.currentMultiUpload.CurrentItem += 1

		err := dlg.Text(fmt.Sprintf("Uploaded frame %d/%d", cItem, total))
		if err != nil {
			return err
		}

		return nil
	}

	if strings.HasPrefix(lastResp, "OK FRAMES READY") {
		sio.messageQueue <- []byte(fmt.Sprintf("SIZE %d\r\n", len(mcu.LoadedFiles[cItem])))
		return nil
	}

	if strings.HasPrefix(lastResp, "OK FRAME NEXT") {
		sio.messageQueue <- []byte(fmt.Sprintf("SIZE %d\r\n", len(mcu.LoadedFiles[cItem])))
		return nil
	}

	end := util.MinInt(mcu.TotalBytesSent[cItem]+UploadBlock, len(mcu.LoadedFiles[cItem]))
	sio.messageQueue <- mcu.LoadedFiles[cItem][mcu.TotalBytesSent[cItem]:end]
	mcu.TotalBytesSent[cItem] = end

	err := dlg.Value(int((float64(cItem) / float64(total)) * 100))
	err = dlg.Text(fmt.Sprintf("Uploaded frame %d/%d", cItem, total))

	if err != nil {
		logger.Warnw("Issues updating progress dialog", "error", err)
	}

	logger.Debugw("Sent bytes to controller", "cItem", cItem, "total", total, "sentBytes", mcu.TotalBytesSent[cItem], "totalBytes", len(mcu.LoadedFiles[cItem]))

	return nil
}

func (sio *SerialIO) handleTransferError(logger *zap.SugaredLogger, line string) {
	cleaned := strings.TrimSuffix(line, "\r\n")
	split := strings.Split(cleaned, ",")

	reason := "Unknown"
	if len(split) > 1 {
		reason = split[1]
	} else {
		logger.Warnw("Controller did not give a reason for failure", "line", split)
	}

	logger.Errorw("Failed to transfer image to microcontroller", "reason", reason)

	dlg := *sio.transferDialog
	err := dlg.Text(fmt.Sprintf("Transfer FAILED! Reason: %s", reason))
	err = dlg.Complete()

	if err != nil {
		logger.Warnw("Failed to finish dialogue")
	}
}
