import machine
import utime

# Deej vars
NUM_SLIDERS = 2
NUM_SWITCHES = 3

analogSliderValues = [0, 0]
switchValues = [
    machine.Pin(2, machine.Pin.IN, machine.Pin.PULL_UP),
    machine.Pin(3, machine.Pin.IN, machine.Pin.PULL_UP),
    machine.Pin(4, machine.Pin.IN, machine.Pin.PULL_UP) # Change depending on what type of switches you have
]

slideMaster = machine.ADC(26)
slideProg1 = machine.ADC(27)

# Read each slider's position
def updateSliderValues():
    for x in range(NUM_SLIDERS):
        if(x == 0):
            analogSliderValues[0] = slideMaster.read_u16() //64
        elif(x == 1):
            analogSliderValues[1] = slideProg1.read_u16() //64

# Make a string with all slider values and print it
def sendSliderValues():
    builtString = ""
    builtString2 = ""
    
    for x in range(NUM_SLIDERS):
        builtString +=  str(analogSliderValues[x])
        
        if(x < NUM_SLIDERS - 1):
            builtString += '|'
    
    for z in range(NUM_SWITCHES):
        builtString2 += str(1 - switchValues[z].value()) # Change depending on what type of switches you have
        
        if (z < NUM_SWITCHES - 1):
            builtString2 += '|'
    
    print(f"{builtString} {builtString2}")
  
while(True):
    updateSliderValues()
    sendSliderValues()
    utime.sleep(0.01)
