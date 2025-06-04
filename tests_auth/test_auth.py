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
        dbname="auth",
        user="admin",
        password="superpass",
        host="postgres",
        port="5432",
        row_factory=dict_row
    )
    yield conn
    conn.close()

def register_user(data):
    url = f"{BASE_URL}{API_PREFIX}/register"
    response = requests.post(url, json=data)
    return response.status_code

def login_user(username, password):
    url = f"{BASE_URL}{API_PREFIX}/login"
    data = {
        "username": username,
        "password": password
    }
    response = requests.post(url, json=data)
    assert response.status_code == 200
    jwt_token = response.cookies.get('jwt')
    return jwt_token

def whoami(jwt_token):
    url = f"{BASE_URL}{API_PREFIX}/whoami"
    cookies = {'jwt': jwt_token}

    response = requests.post(url, cookies=cookies)
    assert response.status_code == 200
    return response.text

def test_simple_register(db_connection):
    username = 'username'
    mail = 'mail@mail.ru'
    password = 'password'
    assert register_user({
        "username": username,
        "mail": mail,
        "password": password
    }) == 200
    cursor = db_connection.cursor()
    cursor.execute("SELECT * FROM users")
    result = cursor.fetchone()
    assert result['id'] == 1
    assert result['username'] == username
    assert result['mail'] == mail

def test_register_no_mail(db_connection):
    username = 'username_no_mail'
    password = 'password'
    assert register_user({
        "username": username,
        "password": password
    }) == 400
    cursor = db_connection.cursor()
    cursor.execute(f"SELECT * FROM users WHERE username='{username}'")
    result = cursor.fetchall()
    assert result == []


def test_register_no_password(db_connection):
    username = 'username_no_pass'
    mail = 'mail123@mail.ru'
    assert register_user({
        "username": username,
        "mail": mail
    }) == 400
    cursor = db_connection.cursor()
    cursor.execute(f"SELECT * FROM users WHERE username='{username}'")
    result = cursor.fetchall()
    assert result == []

def test_duplicate_username(db_connection):
    username = 'username'
    mail = 'mail_new@mail.ru'
    real_mail = 'mail@mail.ru'
    password = 'password'
    assert register_user({
        "username": username,
        "mail": mail,
        "password": password
    }) == 400
    cursor = db_connection.cursor()
    cursor.execute("SELECT * FROM users")
    results = cursor.fetchall()
    assert len(results) == 1
    result = results[0]
    assert result['id'] == 1
    assert result['username'] == username
    assert result['mail'] == real_mail


def test_login_and_whoami():
    username = 'username'
    password = 'password'
    jwt = login_user(username, password)
    assert whoami(jwt) == ''

