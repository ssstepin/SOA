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
        dbname="statistics",
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

def like_post(jwt_token, post_id):
    url = f"{BASE_URL}{API_PREFIX}/posts/like"
    cookies = {'jwt': jwt_token}
    data = {
        "post_id": post_id
    }
    response = requests.post(url, cookies=cookies, json=data)
    return response.status_code == 200

def view_post(jwt_token, post_id):
    url = f"{BASE_URL}{API_PREFIX}/posts/view"
    cookies = {'jwt': jwt_token}
    data = {
        "post_id": post_id
    }
    response = requests.post(url, cookies=cookies, json=data)
    return response.status_code == 200

def comment_post(jwt_token, post_id, text):
    url = f"{BASE_URL}{API_PREFIX}/posts/comment"
    cookies = {'jwt': jwt_token}
    data = {
        "post_id": post_id,
        "text": text
    }
    response = requests.post(url, cookies=cookies, json=data)
    return response.status_code == 200

def get_top_users(metric="likes", limit=10):
    url = f"{BASE_URL}{API_PREFIX}/stats/top/users?metric={metric}&limit={limit}"
    response = requests.get(url)
    return response.json()

def test_like(db_connection):
    username = 'username'
    password = 'password'
    register_user(username, password) # first time
    jwt = login_user(username, password)
    text = 'test text'
    post_id = create_post(jwt, text)['post_id']
    assert like_post(jwt, post_id)

    time.sleep(3.0)
    cursor = db_connection.cursor()
    cursor.execute(f"SELECT * FROM events WHERE post_id='{post_id}'")
    result = cursor.fetchall()
    assert len(result) == 1
    post = result[0]
    assert post['event_type'] == 'like'
    assert post['user_id'] == 1
    assert post['post_id'] == post_id

def test_view(db_connection):
    username = 'username'
    password = 'password'
    jwt = login_user(username, password)
    text = 'test text'
    post_id = create_post(jwt, text)['post_id']
    assert view_post(jwt, post_id)

    time.sleep(3.0)
    cursor = db_connection.cursor()
    cursor.execute(f"SELECT * FROM events WHERE post_id='{post_id}'")
    result = cursor.fetchall()
    assert len(result) == 1
    post = result[0]
    assert post['event_type'] == 'view'
    assert post['user_id'] == 1
    assert post['post_id'] == post_id

def test_comment(db_connection):
    username = 'username'
    password = 'password'
    jwt = login_user(username, password)
    text = 'test text'
    post_id = create_post(jwt, text)['post_id']
    comment = "aaa"
    assert comment_post(jwt, post_id, comment)

    time.sleep(3.0)
    cursor = db_connection.cursor()
    cursor.execute(f"SELECT * FROM events WHERE post_id='{post_id}'")
    result = cursor.fetchall()
    assert len(result) == 1
    post = result[0]
    assert post['event_type'] == 'comment'
    assert post['user_id'] == 1
    assert post['post_id'] == post_id
    assert post['comment_text'] == comment

def test_top_users():
    for metric in ["likes", "views", "comments"]:
        top = get_top_users(metric=metric, limit=5)
        assert top['users'] == [{'user_id': 1, 'count': 1}]