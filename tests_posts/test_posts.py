import pytest
import requests
import time
import psycopg
from psycopg.rows import dict_row

BASE_URL = "http://gateway:8080"
API_PREFIX = "/api/v1"

@pytest.fixture
def db_connection():
    # Подключение с использованием psycopg3
    conn = psycopg.connect(
        dbname="posts",
        user="admin",
        password="superpass",
        host="postgres",
        port="5432",
        row_factory=dict_row
    )
    yield conn
    conn.close()

def register_user(username, password):
    url = f"{BASE_URL}{API_PREFIX}/register"
    data = {
        "username": username,
        "mail": username + "@mail.ru",
        "password": password
    }
    response = requests.post(url, json=data)

def login_user(username, password):
    url = f"{BASE_URL}{API_PREFIX}/login"
    data = {
        "username": username,
        "password": password
    }
    response = requests.post(url, json=data)
    jwt_token = response.cookies.get('jwt')
    return jwt_token

def create_post(jwt_token, text):
    url = f"{BASE_URL}{API_PREFIX}/posts/create"
    cookies = {'jwt': jwt_token}
    data = {
        "text": text
    }
    response = requests.post(url, cookies=cookies, json=data)
    return response.json()

def test_simple_create(db_connection):
    username = 'username'
    password = 'password'
    register_user(username, password)
    jwt = login_user(username, password)
    text = 'test text'
    post = create_post(jwt, text)
    assert post['post_id'] == 1
    assert post['text'] == text
    assert post['user_id'] == 1

    cursor = db_connection.cursor()
    cursor.execute(f"SELECT * FROM posts")
    result = cursor.fetchall()
    assert len(result) == 1
    post = result[0]
    assert post['id'] == 1
    assert post['text'] == text
    assert post['user_id'] == 1


def test_like_dryrun():
    username = 'username'
    password = 'password'
    jwt = login_user(username, password)
    url = f"{BASE_URL}{API_PREFIX}/posts/like"
    cookies = {'jwt': jwt}
    data = {
        "post_id": 1
    }
    response = requests.post(url, cookies=cookies, json=data)
    assert response.status_code == 200

def test_view_dryrun():
    username = 'username'
    password = 'password'
    jwt = login_user(username, password)
    url = f"{BASE_URL}{API_PREFIX}/posts/view"
    cookies = {'jwt': jwt}
    data = {
        "post_id": 1
    }
    response = requests.post(url, cookies=cookies, json=data)
    assert response.status_code == 200

def test_comment_dryrun():
    username = 'username'
    password = 'password'
    jwt = login_user(username, password)
    url = f"{BASE_URL}{API_PREFIX}/posts/comment"
    cookies = {'jwt': jwt}
    data = {
        "post_id": 1,
        'text': 'i am comment'
    }
    response = requests.post(url, cookies=cookies, json=data)
    assert response.status_code == 200