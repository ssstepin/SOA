import pytest
import requests
from kafka import KafkaProducer, KafkaConsumer, KafkaAdminClient
import time

BASE_URL = "http://gateway:8080"
API_PREFIX = "/api/v1"

@pytest.fixture
def kafka_consumer():
    consumer = KafkaConsumer(
        bootstrap_servers=['kafka:9092'],
        auto_offset_reset='earliest',
        consumer_timeout_ms=20000,
        enable_auto_commit=False,
        group_id='test-group',
    )
    yield consumer
    consumer.close()

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

def test_view_message_produced_to_kafka(kafka_consumer):
    try:
        admin = KafkaAdminClient(bootstrap_servers='kafka:9092', request_timeout_ms=3000)
    except Exception as e:
        pytest.fail(f"Kafka недоступна: {str(e)}")
    kafka_consumer.subscribe(['post_views'])
    register_user("user_test_view_consumer", "aaa")
    user_token = login_user("user_test_view_consumer", "aaa")
    post = create_post(user_token, "lorem ipsum")
    post_id = post['post_id']
    assert view_post(user_token, post_id)
    time.sleep(2.0)
    messages = []
    for message in kafka_consumer:
        if message is not None:
            messages.append(message.key)
            break

    assert len(messages) == 1
    assert messages[0] == b'1:1'

def test_like_message_produced_to_kafka(kafka_consumer):
    try:
        admin = KafkaAdminClient(bootstrap_servers='kafka:9092', request_timeout_ms=3000)
    except Exception as e:
        pytest.fail(f"Kafka недоступна: {str(e)}")
    kafka_consumer.subscribe(['post_likes'])
    register_user("user_test_like_consumer", "aaa")
    user_token = login_user("user_test_like_consumer", "aaa")
    post = create_post(user_token, "lorem ipsum")
    post_id = post['post_id']
    assert like_post(user_token, post_id)
    time.sleep(2.0)
    messages = []
    for message in kafka_consumer:
        if message is not None:
            messages.append(message.key)
            break

    assert len(messages) == 1
    assert messages[0] == b'2:2'

def test_comment_message_produced_to_kafka(kafka_consumer):
    try:
        admin = KafkaAdminClient(bootstrap_servers='kafka:9092', request_timeout_ms=3000)
    except Exception as e:
        pytest.fail(f"Kafka недоступна: {str(e)}")
    kafka_consumer.subscribe(['post_comments'])
    register_user("user_test_comment_consumer", "aaa")
    user_token = login_user("user_test_comment_consumer", "aaa")
    post = create_post(user_token, "lorem ipsum")
    post_id = post['post_id']
    assert comment_post(user_token, post_id, 'test_text')
    time.sleep(2.0)
    messages = []
    for message in kafka_consumer:
        if message is not None:
            messages.append(message.key)
            break

    assert len(messages) == 1
    assert messages[0] == b'3:3:test_text'