import serial
import time
import sys

# Load the image as binary
with open("pearto.png", "rb") as f:
    data = f.read()

ser = serial.Serial('COM3', 115200, timeout=2)
ser.write(f"sendimg {len(data)}".encode())

# Wait for confirmation from the microcontroller
response = ser.readline().decode().strip()
print("Microcontroller response:", response)

if not response.startswith("OK"):
    print("Microcontroller not ready, aborting.")
    ser.close()
    exit()

time.sleep(0.5)

# Send data in chunks
total_sent = 0
while total_sent < len(data):
    end = min(total_sent + 1024, len(data))
    ser.write(data[total_sent:end])
    total_sent = end
    print(f"Total bytes sent: {total_sent}")
    resp = ser.readline().decode().strip()
    print(resp)
    if "OK" not in resp:
        print("OK not received for chunk")
        sys.exit(1)
    time.sleep(0.1)

print("Upload complete")

# Read all remaining lines (the "OK, data fully received" etc.)
for _ in range(5):
    line = ser.readline()
    if not line:
        break
    print(">>", line.decode().strip())

ser.close()