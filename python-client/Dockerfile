FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Inside a Docker Compose network the Go wrapper is reachable on its internal
# port (8080). Override GO_API_URL if using a different network topology.
ENV GO_API_URL=http://bitso-wrapper:8080 \
    SCHEDULER_INTERVAL_MINUTES=5

CMD ["python", "main.py"]
