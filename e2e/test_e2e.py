import pytest
import requests
import time


BASE_URL = "http://gateway:8080"
API_PREFIX = "/api/v1"

def register_user(username, password):
    url = f"{BASE_URL}{API_PREFIX}/register"
    data = {
        "username": username,
        "mail": username + "@mail.ru",
        "password": password
    }
    response = requests.post(url, json=data)
    assert response.status_code == 200

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

def create_post(jwt_token, text):
    url = f"{BASE_URL}{API_PREFIX}/posts/create"
    cookies = {'jwt': jwt_token}
    data = {
        "text": text
    }
    response = requests.post(url, cookies=cookies, json=data)
    assert response.status_code == 200
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

def get_post_stats(post_id):
    url = f"{BASE_URL}{API_PREFIX}/stats/post/{post_id}"
    response = requests.get(url)
    return response.json()

def get_top_posts(metric="likes", limit=10):
    url = f"{BASE_URL}{API_PREFIX}/stats/top/posts?metric={metric}&limit={limit}"
    response = requests.get(url)
    return response.json()


def test_chaos():
    jwt_dict = {}
    posts_dict = {}
    post_ids = []
    N = 5
    for i in range(1, N + 1):
        username = f'user_num{i}'
        password = f'pass{i}'
        register_user(username, password)
        jwt = login_user(username, password)
        jwt_dict[i] = jwt
        post = create_post(jwt, 'hiii!!!!!')
        post_id = post['post_id']
        post_ids.append(post_id)
        posts_dict[post_id] = {
            'comments': 0,
            'views': 0,
            'likes': 0
        }

    for i in range(1, N + 1):
        jwt = jwt_dict[i]
        for j in range(1, i + 1):
            post_id = post_ids[j - 1]
            assert view_post(jwt, post_id)
            posts_dict[post_id]['views'] += 1
            assert like_post(jwt, post_id)
            posts_dict[post_id]['likes'] += 1
            assert comment_post(jwt, post_id, 'wow')
            posts_dict[post_id]['comments'] += 1

    for post_id in post_ids:
        stats = get_post_stats(post_id)
        assert stats == posts_dict[post_id]

    for metric in ["likes", "views", "comments"]:
        top = get_top_posts(metric=metric, limit=5)['posts']
        exp = [{'post_id': post_ids[i], 'count': N - i} for i in range(N)]
        #exp = []
        assert top == exp



def test_1_user_basic():
    username = "username"
    password = "password"
    register_user(username, password)
    jwt = login_user(username, password)
    text = "my first post!"
    post = create_post(jwt, text)
    assert post == {
        'post_id': 6,
        'user_id': 6,
        'text': text
    }


def test_2_users_interact():
    first_username = "username1"
    first_password = "password1"
    register_user(first_username, first_password)
    first_jwt = login_user(first_username, first_password)
    first_text = "Good post"
    first_post = create_post(first_jwt, first_text)
    first_post_id = first_post['post_id']

    second_username = "username2"
    second_password = "password2"
    register_user(second_username, second_password)
    second_jwt = login_user(second_username, second_password)
    second_text = "Very good post"
    second_post = create_post(second_jwt, second_text)
    second_post_id = second_post['post_id']

    assert view_post(first_jwt, second_post_id)
    assert like_post(first_jwt, second_post_id)
    time.sleep(3.0) # for kafka
    assert get_post_stats(second_post_id) == {
        'likes': 1,
        'views': 1
    }

    assert view_post(second_jwt, first_post_id)
    assert comment_post(second_jwt, first_post_id, 'wow')
    time.sleep(3.0) # for kafka
    assert get_post_stats(first_post_id) == {
        'comments': 1,
        'views': 1
    }