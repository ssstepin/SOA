FROM python:3.9-alpine

WORKDIR /tests_posts

COPY requirements.txt .
RUN pip install -r requirements.txt

COPY *.py .

ENTRYPOINT pytest -vv