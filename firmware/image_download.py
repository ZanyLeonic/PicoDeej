from proj_const import *
import os
import gc
import micropython
import sys

def download_static_image(cmd, tft):
    if len(cmd) < 2 or not cmd[1].isdigit():
        print("FAIL, invalid arguments")
        return
    
    size = int(cmd[1])
    if size > MAX_FILE_SIZE:
        print("FAIL, too big")
        return
    
    print(f"OK READY {size}")
    
    micropython.kbd_intr(-1) # Disable keyboard interrupt whilst sending binary
    
    gc.collect()
    
    total_recv = 0
    with open(UPLOAD_PARTIAL, "wb") as f:
        while total_recv < size:
            remaining = size - total_recv
            
            chunk = min(1024, remaining)
            f.write(sys.stdin.buffer.read(chunk))
            
            total_recv += chunk
            print(f"OK {total_recv}")
    
    micropython.kbd_intr(0x03) # Re-enable Keyboard interrupt
    
    print(f"OK DONE {total_recv}")

    os.remove(UPLOAD_IMAGE)
    os.rename(UPLOAD_PARTIAL, UPLOAD_IMAGE)
    os.sync()
    
    gc.collect()

def download_animated_set(cmd):
    if len(cmd) < 2 or not cmd[1].isdigit():
        print("FAIL, invalid arguments")
        return

    nFrames = int(cmd[1])

    if nFrames > MAX_FRAMES:
        print("FAIL, too many frames")
        return
    STOP_ANIMATION_THREAD = True
    eFiles = None
    try:
        eFiles = os.listdir(ANIM_FOLDER)
    except:
        pass

    if eFiles == None:
        try:
            os.mkdir(ANIM_FOLDER)
        except:
            print("FAIL, cannot create folder for animations")
            return

    if eFiles != None:
        try:
            for file in eFiles:
                os.remove(f"{ANIM_FOLDER}/{file}")
        except:
            print("FAIL, cannot clear old animation directory")
            return

    print(f"OK FRAMES READY {nFrames}")

    for nF in range(nFrames):
        total_recv = 0
        if not nF == 0:
            print(f"OK FRAME NEXT {nF}")
        
        # Read header as binary, then decode
        header_line = sys.stdin.buffer.readline()
        try:
            header_text = header_line.decode('utf-8').strip()
        except UnicodeDecodeError:
            print("FAIL, header is not valid UTF-8")
            return
        
        rawSize = header_text.split(" ")
        if len(rawSize) < 2 or not rawSize[1].isdigit():
            print("FAIL, frame size incorrectly specified")
            return
        
        size = int(rawSize[1])
        
        print(f"OK FRAME READY {size}")
        
        micropython.kbd_intr(-1) # Disable keyboard interrupt whilst sending binary
        
        with open(UPLOAD_PARTIAL, "wb") as f:
            while total_recv < size:
                remaining = size - total_recv
                
                chunk = min(1024, remaining)
                f.write(sys.stdin.buffer.read(chunk))
                
                total_recv += chunk
                print(f"OK FRAME {total_recv}")

        os.rename(UPLOAD_PARTIAL, f"{ANIM_FOLDER}/frame_{nF:03}.png")
        os.sync()
        
        micropython.kbd_intr(0x03)
                    
    print(f"OK DONE FRAMES {nFrames}")