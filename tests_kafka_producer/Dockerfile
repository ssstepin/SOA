FROM python:3.9-alpine

WORKDIR /tests

COPY requirements.txt .
RUN pip install -r requirements.txt

COPY *.py .

ENTRYPOINT pytest -v