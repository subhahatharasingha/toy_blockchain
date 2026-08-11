# Create data directories
New-Item -ItemType Directory -Force -Path "data/nodeA"
New-Item -ItemType Directory -Force -Path "data/nodeB"
New-Item -ItemType Directory -Force -Path "data/nodeC"

# Start Node A
Write-Host "Starting Node A on Port 8001..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run . node --id A --host 127.0.0.1 --port 8001 --file data/nodeA/blockchain.json"

# Wait a moment for Node A to bind
Start-Sleep -Seconds 2

# Start Node B
Write-Host "Starting Node B on Port 8002..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run . node --id B --host 127.0.0.1 --port 8002 --file data/nodeB/blockchain.json --peers http://127.0.0.1:8001"

# Start Node C
Write-Host "Starting Node C on Port 8003..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run . node --id C --host 127.0.0.1 --port 8003 --file data/nodeC/blockchain.json --peers http://127.0.0.1:8001,http://127.0.0.1:8002"

Write-Host "All nodes launched successfully in separate windows." -ForegroundColor Cyan
