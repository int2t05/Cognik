# 认证与权限

> 覆盖登录双令牌、Token 刷新、中间件链、RBAC、用户/角色/菜单管理全链路。

---

## 1. 登录全链路（username+password → JWT Pair + 权限数据）

```mermaid
flowchart TB
    subgraph Input["输入"]
        I1["POST /api/v1/auth/login<br/><b>username</b> : string<br/><b>password</b> : string"]
    end

    subgraph Handler["AuthHandler.Login — handler/auth.go"]
        H1["c.ShouldBindJSON(&LoginRequest)"]
    end

    subgraph Service["AuthService.Login — service/auth_service.go"]
        S1["UserRepo.GetByUsername(username)<br/>SELECT * FROM users WHERE username=?"]
        S2["密码校验<br/>bcrypt.CompareHashAndPassword(hash, password)<br/>失败 → AppError{10001}"]
        S3["状态校验<br/>User.Status == Active(1)<br/>冻结 → AppError{10006}"]
        S4["JWT 双令牌生成<br/>jwt.GenerateAccessToken(userID, username)<br/>  Claims{TokenType:'access', ExpiresAt:now+2h}<br/>jwt.GenerateRefreshToken(userID, username)<br/>  Claims{TokenType:'refresh', ExpiresAt:now+168h}"]
        S5["权限数据组装<br/>UserRepo.GetUserPermissions(userID)<br/>  → roles + permissions<br/>RoleRepo.BatchGetRoleMenus(roleIDs)<br/>  → buildTree(menus, 0) — 菜单树"]
    end

    subgraph Output["输出"]
        O1["200 {access_token, refresh_token, user, roles, permissions, menus}"]
    end

    I1 --> H1 --> S1 --> S2 --> S3 --> S4 --> S5 --> O1

    style Input fill:#1e293b,stroke:#334155,color:#e2e8f0
    style Output fill:#1e293b,stroke:#334155,color:#e2e8f0
    style S4 fill:#f59e0b10,stroke:#f59e0b
```

---

## 2. 登录时序详解

```mermaid
sequenceDiagram
    actor U as 用户
    participant AH as AuthHandler.Login
    participant AS as AuthService.Login
    participant UR as UserRepo.GetByUsername
    participant JWT as pkg/jwt
    participant DB as PostgreSQL

    U->>AH: POST /api/v1/auth/login {username, password}
    AH->>AH: c.ShouldBindJSON(&LoginRequest)
    AH->>AS: Login(username, password)

    AS->>UR: GetByUsername(username)
    UR->>DB: SELECT * FROM users WHERE username=?
    DB-->>UR: *User{PasswordHash, Status}
    UR-->>AS: *User

    AS->>AS: bcrypt.CompareHashAndPassword(hash, password)
    alt 密码错误
        AS-->>AH: AppError{10001, "用户名或密码错误"}
        AH-->>U: 401
    end

    AS->>AS: 状态检查: Status == Active(1)

    AS->>JWT: GenerateAccessToken(userID, username)
    JWT->>JWT: Claims{TokenType:"access", ExpiresAt:now+2h}
    JWT-->>AS: accessToken

    AS->>JWT: GenerateRefreshToken(userID, username)
    JWT->>JWT: Claims{TokenType:"refresh", ExpiresAt:now+168h}
    JWT-->>AS: refreshToken

    AS-->>AH: *LoginResponse{AccessToken, RefreshToken, User, Roles, Permissions, Menus}
    AH-->>U: 200
```

---

## 3. Token 刷新链路

```mermaid
flowchart LR
    IN["POST /api/v1/auth/refresh<br/><b>refresh_token</b>"] --> H["AuthHandler.Refresh"]
    H --> J1["jwt.ParseWithClaims(refreshToken, &Claims{})"]
    J1 --> CHK{"claims.TokenType<br/>== 'refresh'?"}
    CHK -->|否| ERR["401 无效令牌类型"]
    CHK -->|是| J2["jwt.GenerateAccessToken(claims.UserID, claims.Username)"]
    J2 --> J3["jwt.GenerateRefreshToken(claims.UserID, claims.Username)"]
    J3 --> OUT["200 {access_token, refresh_token}"]

    style IN fill:#1e293b,stroke:#334155,color:#e2e8f0
    style OUT fill:#1e293b,stroke:#334155,color:#e2e8f0
    style ERR fill:#ef444420,stroke:#ef4444
```

---

## 4. 中间件认证链

```mermaid
flowchart TB
    REQ["HTTP Request<br/>Authorization: Bearer &lt;token&gt;"] --> M1

    subgraph Middleware["中间件链 — router.go:Setup"]
        M1["Recovery — panic 恢复"]
        M2["RequestID — c.Set('requestID', uuid)"]
        M3["CORS — AllowOrigin 校验 + OPTIONS 预检"]
        M4["Logger — 记录 method/path/status/latency"]
    end

    subgraph JWT["JWTAuth — middleware/auth.go"]
        J1["c.GetHeader('Authorization')"]
        J2["strings.TrimPrefix('Bearer ', token)"]
        J3{"token != ''?"}
        J3 -->|否| E401A["401 {code:10001}"]
        J4["jwt.ParseWithClaims(token, &Claims{}, keyFunc)"]
        J5{"解析成功?"}
        J5 -->|否| E401B["401 {code:10001}"]
        J6{"claims.TokenType == 'access'?"}
        J6 -->|否 (refresh_token)| E401C["401 {code:10001}"]
        J7["c.Set('userID', claims.UserID)<br/>c.Set('username', claims.Username)<br/>c.Next()"]
    end

    subgraph RBAC["RequirePermission — middleware/rbac.go"]
        R1{"Admin 路由组?"}
        R1 -->|否| R4["c.Next() → Handler"]
        R2["c.Get('userPermissions').([]string)"]
        R3{"requiredPermission ∈ userPermissions?"}
        R3 -->|否| E403["403 {code:10002}"]
    end

    REQ --> M1 --> M2 --> M3 --> M4
    M4 --> J1 --> J2 --> J3
    J3 -->|是| J4 --> J5
    J5 -->|是| J6
    J6 -->|是| J7 --> R1
    R1 -->|是| R2 --> R3
    R3 -->|是| R4

    style E401A fill:#ef444420,stroke:#ef4444
    style E401B fill:#ef444420,stroke:#ef4444
    style E401C fill:#ef444420,stroke:#ef4444
    style E403 fill:#ef444420,stroke:#ef4444
```

---

## 5. 路由分组与权限映射

```mermaid
flowchart TD
    R["gin.Engine :8080"]
    R --> Public["/api/v1/auth — 无中间件"]
    R --> AuthMe["/api/v1/auth/me — JWTAuth"]
    R --> Portal["/api/v1/portal — JWTAuth"]
    R --> Admin["/api/v1/admin — JWTAuth + RBAC"]

    Public --> P1["POST /login → AuthHandler.Login"]
    Public --> P2["POST /refresh → AuthHandler.Refresh"]

    AuthMe --> A1["POST /change-password → AuthHandler.ChangePassword"]
    AuthMe --> A2["POST /logout → AuthHandler.Logout"]

    Portal --> PT["chat-sessions / tickets / messages<br/>→ ChatHandler / TicketHandler / MessageHandler"]

    Admin --> AD["tickets / knowledge-bases / users / roles<br/>/ llm-configs / dashboard / audit-logs / configs"]

    style Public fill:#22c55e15,stroke:#22c55e
    style AuthMe fill:#f59e0b15,stroke:#f59e0b
    style Portal fill:#5e6ad215,stroke:#5e6ad2
    style Admin fill:#ef444415,stroke:#ef4444
```

---

## 6. 用户管理数据流

```mermaid
flowchart TB
    subgraph Create["创建用户"]
        C1["UserHandler.Create(c)<br/>c.ShouldBindJSON(&CreateUserRequest)<br/>{username, password, role_ids}"]
        C2["UserService.Create(req)<br/>└─ UserRepo.ExistsByUsername — 唯一性检查<br/>└─ ValidatePassword — ^(?=.*[a-z])(?=.*[A-Z])(?=.*\\d).{8,32}$<br/>└─ HashPassword — bcrypt cost=10<br/>└─ UserRepo.Create(&User{Status:1, FirstLogin:true})<br/>└─ UserRepo.AssignRoles(userID, roleIDs)"]
        C3[("INSERT INTO users<br/>INSERT INTO user_roles")]
    end

    subgraph Freeze["冻结/恢复"]
        F1["UserHandler.Freeze(c) / Restore(c)"]
        F2["UserService.Freeze(id) → 状态检查: 已冻结→10006<br/>UserService.Restore(id) → 状态检查: 已正常→10007"]
        F3["UserRepo.UpdateStatus(id, status) — 立即生效（无缓存）"]
    end

    C1 --> C2 --> C3
    F1 --> F2 --> F3

    style Create fill:#22c55e10,stroke:#22c55e
    style Freeze fill:#f59e0b10,stroke:#f59e0b
```

---

## 7. 角色与菜单管理

```mermaid
flowchart TB
    subgraph Role["角色 CRUD"]
        RC1["RoleHandler.Create(c)<br/>{name, description, permissions[], menu_ids[]}"]
        RC2["RoleService.Create(name, description, permissions)<br/>└─ RoleRepo.CreateRole(&Role{Permissions:JSONB})<br/>└─ RoleRepo.UpdateRoleMenus(roleID, menuIDs)<br/>    DELETE + INSERT role_menus — 全量替换"]
        RC3[("INSERT INTO roles<br/>INSERT INTO role_menus")]
    end

    subgraph Menu["菜单树构建"]
        MT1["RoleHandler.ListMenus(c) → GET /admin/menus"]
        MT2["RoleService.ListMenus()<br/>└─ RoleRepo.ListMenus()<br/>    SELECT * FROM menus ORDER BY sort_order<br/>└─ buildTree(menus, 0) — 递归构建嵌套树"]
    end

    subgraph Bind["角色菜单绑定"]
        RM1["RoleHandler.UpdateRoleMenus(c)<br/>PUT /admin/roles/:id/menus {menu_ids[]}"]
        RM2["RoleService.UpdateRoleMenus(roleID, menuIDs)<br/>└─ RoleRepo.UpdateRoleMenus — 全量替换策略"]
    end

    RC1 --> RC2 --> RC3
    MT1 --> MT2
    RM1 --> RM2

    style Role fill:#22c55e10,stroke:#22c55e
    style Menu fill:#5e6ad210,stroke:#5e6ad2
    style Bind fill:#f59e0b10,stroke:#f59e0b
```

---

> 相关文件：`server/internal/handler/auth.go` / `server/internal/middleware/auth.go` / `server/internal/middleware/rbac.go` / `server/internal/service/auth_service.go` / `server/internal/service/user_service.go` / `server/internal/service/role_service.go` / `server/pkg/jwt/jwt.go`
