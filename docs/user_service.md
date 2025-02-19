```mermaid
erDiagram
    user {
        int id PK
        string name
        string surname
        string login
        timestamp created 
        bson meta
    }

    user_auth_info {
        int user_id FK
        string email
        string phone
        string hashed_password
        string confirmed
    }


     user_sessions {
        int id PK
        int user_id FK
        timestamp start
        timestamp end
        string device_model
     }

     user_roles_info {
        int id PK
        int parent_id FK
        string name
        text description
     }

     user_roles_items {
        int id PK
        int role_id FK
        int user_id FK
        timestamp granted_ts
        timestamp due_ts
     }

     user ||--|| user_auth_info : asosciated
     user ||--o{ user_sessions: created
     user ||--o{ user_roles_items: "have roles"
     user_roles_info ||--o{ user_roles_items: granted
```