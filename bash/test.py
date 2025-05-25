import requests
from faker import Faker
import random
import time
import json
from datetime import datetime, timedelta

# Конфигурация
BASE_URL = "http://localhost:8080"
API_PREFIX = "/api/v1"
FAKER = Faker()

# Глобальные переменные для хранения данных
users = []
posts = []

def register_user(username, password):
    url = f"{BASE_URL}{API_PREFIX}/register"
    data = {
        "username": username,
        "mail": username + "@mail.ru",
        "password": password
    }
    response = requests.post(url, json=data)
    print(response)


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

def get_post_stats(post_id):
    url = f"{BASE_URL}{API_PREFIX}/stats/post/{post_id}"
    response = requests.get(url)
    return response.json()

def get_post_trends(post_id, days=7):
    url = f"{BASE_URL}{API_PREFIX}/stats/trends/{post_id}?days={days}"
    response = requests.get(url)
    return response.json()

def get_top_posts(metric="likes", limit=10):
    url = f"{BASE_URL}{API_PREFIX}/stats/top/posts?metric={metric}&limit={limit}"
    response = requests.get(url)
    return response.json()

def get_top_users(metric="likes", limit=10):
    url = f"{BASE_URL}{API_PREFIX}/stats/top/users?metric={metric}&limit={limit}"
    response = requests.get(url)
    return response.json()

def generate_test_data():
    print("=== Генерация тестовых данных ===")

    # Создаем 10 пользователей
    for i in range(1, 11):
        username = f"user_{i}"
        password = f"password_{i}"

        # Регистрация и авторизация
        register_user(username, password)
        jwt_token = login_user(username, password)

        # Создаем пост
        post_text = FAKER.sentence()
        post = create_post(jwt_token, post_text)

        users.append({
            "id": i,
            "username": username,
            "jwt": jwt_token,
            "password": password
        })

        posts.append({
            "id": post['post_id'],
            "user_id": i,
            "text": post_text
        })

        print(f"Создан пользователь {username} и пост {post['post_id']}")

    print("\n=== Генерация взаимодействий ===")

    # Для каждого поста генерируем лайки, просмотры и комментарии
    for i, post in enumerate(posts, 1):
        post_id = post['id']
        interactions_count = i  # 1, 2, 3, ..., 10

        print(f"\nПост {post_id} (автор: user_{post['user_id']}):")
        print(f"Будет {interactions_count} лайков, просмотров и комментариев")

        # Генерируем лайки
        for j in range(interactions_count):
            user = random.choice(users)
            like_post(user['jwt'], post_id)
            time.sleep(0.1)  # Чтобы даты немного различались

        # Генерируем просмотры
        for j in range(interactions_count):
            user = random.choice(users)
            view_post(user['jwt'], post_id)
            time.sleep(0.1)

        # Генерируем комментарии
        for j in range(interactions_count):
            user = random.choice(users)
            comment_text = FAKER.sentence()
            comment_post(user['jwt'], post_id, comment_text)
            time.sleep(0.1)

def test_statistics():
    print("\n=== Тестирование статистики ===")

    # 1. Статистика по каждому посту
    print("\n1. Статистика по постам:")
    for post in posts:
        stats = get_post_stats(post['id'])
        print(f"Пост {post['id']}: {json.dumps(stats, indent=2)}")
        time.sleep(0.5)

    # 2. Динамика по каждому посту
    print("\n2. Динамика по постам:")
    for post in posts:
        trends = get_post_trends(post['id'], days=30)
        print(f"Пост {post['id']}: {len(trends['trends'])} дней статистики")
        time.sleep(0.5)

    # 3. Топ постов по разным метрикам
    print("\n3. Топ постов:")
    for metric in ["likes", "views", "comments"]:
        top = get_top_posts(metric=metric, limit=5)
        print(f"Топ-5 постов по {metric}:")
        for item in top['posts']:
            print(f"  Пост {item['post_id']}: {item['count']}")
        time.sleep(0.5)

    # 4. Топ пользователей по разным метрикам
    print("\n4. Топ пользователей:")
    for metric in ["likes", "views", "comments"]:
        top = get_top_users(metric=metric, limit=5)
        print(f"Топ-5 пользователей по {metric}:")
        for item in top['users']:
            print(f"  Пользователь {item['user_id']}: {item['count']}")
        time.sleep(0.5)

def main():
    # Генерация тестовых данных
    generate_test_data()

    # Тестирование статистики
    test_statistics()

    print("\n=== Тестирование завершено ===")

if __name__ == "__main__":
    main()