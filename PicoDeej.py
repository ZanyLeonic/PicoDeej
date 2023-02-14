import machine
import utime

# Deej vars
NUM_SLIDERS = 3

analogSliderValues = [0, 0, 0]

slideMaster = machine.ADC(26)
slideProg1 = machine.ADC(27)
slideProg2 = machine.ADC(28)

# Read each slider's position
def updateSliderValues():
    for x in range(NUM_SLIDERS):
        if(x == 0):
            analogSliderValues[0] = slideMaster.read_u16()
        elif(x == 1):
            analogSliderValues[1] = slideProg1.read_u16()
        elif(x == 2):
            analogSliderValues[2] = slideProg2.read_u16()

# Make a string with all slider values and print it
def sendSliderValues():
    builtString = ""
    
    for x in range(NUM_SLIDERS):
        builtString +=  str(analogSliderValues[x])
        
        if(x < NUM_SLIDERS - 1):
            builtString += '|'
    
    print(builtString)


while(True):
    updateSliderValues()
    sendSliderValues()
    utime.sleep(0.01)