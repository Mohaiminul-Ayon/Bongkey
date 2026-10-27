# Bongkey Eyewear MVP — Backend Architecture & Complete Flow Guide

> **ডকুমেন্ট টাইপ:** ব্যাকএন্ড আর্কিটেকচার, সিস্টেম ডিজাইন এবং কোড কল-ফ্লো গাইড  
> **প্রযুক্তি:** Go (Golang 1.23+), Gin Web Framework, PostgreSQL (`github.com/lib/pq`), bcrypt, JWT (`golang-jwt/jwt/v5`)  
> **PDF সংস্করণ:** [`docs/backend_architecture_and_flow.pdf`](file:///E:/Eyewear-mvp/docs/backend_architecture_and_flow.pdf)  
> **HTML সংস্করণ:** [`docs/backend_architecture_and_flow.html`](file:///E:/Eyewear-mvp/docs/backend_architecture_and_flow.html)

---

## ১. হাই-লেভেল সিস্টেম ডিজাইন (Layered Clean Architecture)

Bongkey ব্যাকএন্ডটি Go এবং Gin ফ্রেমওয়ার্ক দিয়ে তৈরি একটি Production-grade, **Single Responsibility Principle (SRP)** মেনে চলা আর্কিটেকচার। এখানে কোনো ক্যাচ-অল (`utils`, `helpers`, `common`) প্যাকেজ নেই। প্রতিটি লেয়ারের দায়িত্ব সম্পূর্ণ আলাদা এবং সুনির্দিষ্ট।

```
+----------------------------------------------------------------------------------------------------+
|                                    HTTP CLIENT (Browser / Mobile / cURL)                           |
+----------------------------------------------------------------------------------------------------+
                                                  │
                                                  ▼
+----------------------------------------------------------------------------------------------------+
|                                       TRANSPORT LAYER (HTTP / Gin)                                 |
|  - Routes: routes.SetupRouter(cfg)                                                                 |
|  - Middleware: CORS() ──> AdminAuth()                                                              |
|  - Handlers: Health, Admin, Product, Category, Upload, EmailOTP, Checkout, OrderAccess, OrderList  |
+----------------------------------------------------------------------------------------------------+
                                                  │ (Passes DTOs, Context; No Gin Context to Services)
                                                  ▼
+----------------------------------------------------------------------------------------------------+
|                                       SERVICE LAYER (Business Logic)                               |
|  - auth.Service: Admin password & session verification                                             |
|  - catalog.ProductService: Catalog rules, slugification, stock validation                          |
|  - catalog.CategoryService: Category rules & slug generation                                       |
|  - emailotp.Service: Challenge rate-limiting, code issuance & verification                         |
|  - orders.CheckoutService: Idempotency coordination, stock reservation, price calculation          |
|  - orders.AccessService: Authorization rules for guest tokens vs email proofs vs admin sessions    |
+----------------------------------------------------------------------------------------------------+
                          │                                                │
                          ▼                                                ▼
+----------------------------------------------------+    +------------------------------------------+
|            SECURITY LAYER (Isolated Primitives)    |    |     REPOSITORY LAYER (Persistence / SQL) |
|  - security.IssueAdminToken / VerifyAdminToken     |    |  - catalog.PostgresProductRepository     |
|  - security.IdempotentOrderToken                   |    |  - catalog.PostgresCategoryRepository    |
|  - security.IssueEmailProof / VerifyEmailProof     |    |  - emailotp.PostgresStore                |
|  - security.EmailCodeHash / EmailKey               |    |  - orders.PostgresCheckoutStore (Tx)     |
|  - security.MatchesOrderToken (Constant Time)      |    |  - orders.PostgresReader                 |
+----------------------------------------------------+    |  - auth.PostgresRepository               |
                                                          +------------------------------------------+
                                                                            │
                                                                            ▼
                                                          +------------------------------------------+
                                                          |           POSTGRESQL DATABASE            |
                                                          |  - Connection Pool (database.DB)         |
                                                          |  - Advisory Locks & Constraints          |
                                                          +------------------------------------------+
```

### লেয়ার বাউন্ডারির ৪টি মূল নিয়ম:
1. **হ্যান্ডলার (Handlers):** শুধু HTTP রিকোয়েস্ট পার্স করে (JSON binding, URL params, headers), সার্ভিসকে `context.Context` সহ কল করে এবং ফলাফলকে HTTP Status Code ও JSON-এ ম্যাপ করে। হ্যান্ডলারে **কোনো SQL বা বিজনেস রুল থাকবে না**।
2. **সার্ভিস (Services):** সমস্ত বিজনেস নিয়ম এবং ইনভ্যারিয়েন্ট রক্ষা করে। সার্ভিস কখনো Gin বা HTTP রেসপন্স অবজেক্টের ওপর নির্ভর করে না।
3. **রিপোজিটরি (Repositories):** পিওর SQL এবং ডেটাবেস ট্রানজ্যাকশন এক্সিকিউট করে।
4. **সিকিউরিটি (Security):** ক্রিপ্টোগ্রাফিক অ্যালগরিদম (JWT, HMAC, Constant-Time Compare) আলাদা প্যাকেজে আইসোলেটেড থাকে।

---

## ২. ফাইল ও ফাংশনভিত্তিক বিস্তারিত গাইড (File-by-File Walkthrough)

### ২.১ সার্ভার বুটস্ট্র্যাপিং ও কনফিগারেশন

#### [`backend/cmd/server/main.go`](file:///E:/Eyewear-mvp/backend/cmd/server/main.go)
* [`main()`](file:///E:/Eyewear-mvp/backend/cmd/server/main.go#L10-L18):
  * **কেন আছে:** ব্যাকএন্ডের এন্ট্রি পয়েন্ট।
  * **কী করে:** প্রথমে [`config.Load()`](file:///E:/Eyewear-mvp/backend/internal/config/config.go#L25) কল করে কনফিগ লোড ও ভ্যালিডেট করে। কনফিগ ভ্যালিড হলে [`serve(cfg)`](file:///E:/Eyewear-mvp/backend/cmd/server/server.go#L14) কল করে সার্ভার চালু করে। কোনো ব্যর্থতায় `log.Fatalf` দিয়ে প্রসেস বন্ধ করে।

#### [`backend/cmd/server/server.go`](file:///E:/Eyewear-mvp/backend/cmd/server/server.go)
* [`serve(cfg config.Config) error`](file:///E:/Eyewear-mvp/backend/cmd/server/server.go#L14-L30):
  * **কী করে:** 
    1. `logEnvironment(cfg)` দিয়ে কোন `.env` থেকে ভ্যারিয়েবল নেওয়া হয়েছে তা লগ করে।
    2. [`database.Connect(cfg.DatabaseURL)`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L22) দিয়ে PostgreSQL কানেকশন পুল তৈরি ও পিং টেস্ট করে। ড্রাইভার এররে যাতে পাসওয়ার্ড ফাঁস না হয় সেজন্য জেনেরিক এরর রিটার্ন করে।
    3. [`database.MigrateAndSeed(cfg)`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L40) দিয়ে স্কিমা তৈরি ও সিড করে।
    4. [`routes.SetupRouter(cfg).Run(":" + cfg.ServerPort)`](file:///E:/Eyewear-mvp/backend/internal/routes/routes.go#L17) দিয়ে Gin HTTP লিসেনিং শুরু করে (এটি একটি ব্লকিং কল)।
* [`logEnvironment(cfg config.Config)`](file:///E:/Eyewear-mvp/backend/cmd/server/server.go#L33-L39): কনসোলে নিরাপদে কনফিগ সোর্স প্রিন্ট করে, কিন্তু সিক্রেট ভ্যালু প্রিন্ট করে না।

#### [`backend/internal/config/config.go`](file:///E:/Eyewear-mvp/backend/internal/config/config.go)
* [`Config`](file:///E:/Eyewear-mvp/backend/internal/config/config.go#L11-L23): ডাটাবেস URL, JWT সিক্রেট, জিমেইল ক্রেডেনশিয়াল এবং পোর্ট ধারণকারী স্ট্রাক্ট।
* [`Load() (Config, error)`](file:///E:/Eyewear-mvp/backend/internal/config/config.go#L25-L55): প্রজেক্ট রুট খুঁজে `.env` ফাইল রিড করে ভ্যালু বসায় এবং `validateSecurity()` রান করে।
* [`(cfg Config) validateSecurity() error`](file:///E:/Eyewear-mvp/backend/internal/config/config.go#L58-L66): `JWTSecret` এর দৈর্ঘ্য ন্যূনতম ৩২ ক্যারেক্টার কিনা এবং Gmail ইউজারনেম ও পাসওয়ার্ড একসাথে কনফিগার করা হয়েছে কিনা যাচাই করে। ডিফল্ট টেমপ্লেট সিক্রেট থাকলে সার্ভার স্টার্ট আটকে দেয়।
* [`findProjectRoot()`](file:///E:/Eyewear-mvp/backend/internal/config/config.go#L77-L94): ওপরের ডিরেক্টরিগুলোতে ট্রাভার্স করে যেখানে `.env.example` ও `docker-compose.yml` আছে সেটি রুট হিসেবে চিহ্নিত করে।
* [`loadDotEnv(path string)`](file:///E:/Eyewear-mvp/backend/internal/config/config.go#L96-L137): কাস্টম বাফার স্ক্যানার যা কমেন্ট ও ফাঁকা লাইন বাদ দিয়ে `os.Setenv` করে।

---

### ২.২ ডেটাবেস, স্কিমা ও সিডিং

#### [`backend/internal/database/database.go`](file:///E:/Eyewear-mvp/backend/internal/database/database.go)
* [`DB *sql.DB`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L19): গ্লোবাল কানেকশন পুল।
* [`Connect(databaseURL string) error`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L22-L33): PostgreSQL ড্রাইভার ইনিশিয়ালাইজ করে এবং `db.Ping()` দিয়ে কানেক্টিভিটি টেস্ট করে।
* [`MigrateAndSeed(cfg config.Config) error`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L40-L67):
  * প্রথমে মূল স্কিমা (`schemaSQL`) চালায় (টেবিল: `admins`, `categories`, `products`, `orders`, `order_items`, `invoices`)।
  * ক্রমান্বয়ে ৪টি মাইগ্রেশন ফাইল এক্সিকিউট করে:
    * `002_guest_order_access.sql`: `guest_token_hash` কলাম যোগ করে।
    * `003_stock_quantity_nonnegative.sql`: স্টকে `CHECK (stock_quantity >= 0)` কনস্ট্রেইন্ট দেয়।
    * `004_email_otp.sql`: ওটিপি চ্যালেঞ্জ ও আইপি লিমিট টেবিল তৈরি করে।
    * `005_checkout_idempotency.sql`: `idempotency_key_hash` এর ওপর ইউনিক ইনডেক্স তৈরি করে।
  * এরপর `seedAdmin`, `seedCategories`, এবং `seedProducts` কল করে।
* [`seedAdmin(db, email, password) error`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L70-L100):
  * **Concurrency Protection:** ট্রানজ্যাকশনের শুরুতে `LOCK TABLE admins IN EXCLUSIVE MODE` এক্সিকিউট করে যাতে একাধিক ব্যাকএন্ড ইনস্ট্যান্স একসাথে বুট করলেও একের অধিক ওনার অ্যাকাউন্ট তৈরি হতে না পারে।
  * ওনার একাউন্ট না থাকলে bcrypt দিয়ে পাসওয়ার্ড হ্যাশ করে `role = 'owner'` হিসেবে ইনসার্ট করে।
* [`seedCategories()`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L102-L110) ও [`seedProducts()`](file:///E:/Eyewear-mvp/backend/internal/database/database.go#L121-L162): ডেমো ফ্রেম ও সানগ্লাস প্রোডাক্টগুলো স্পেসিফিকেশনসহ ইনসার্ট করে।
* Seed code slug বানাতে [`catalog.Slugify`](../backend/internal/catalog/products.go) ব্যবহার করে — নাম থেকে স্পেশাল ক্যারেক্টার ও স্পেস সরিয়ে ক্লিন URL স্লাগ (Audit fix 13-এ duplicate সরানো)।

---

### ২.৩ সিকিউরিটি সাবসিস্টেম (Dedicated Security Boundary)

#### [`backend/internal/security/admin_token.go`](file:///E:/Eyewear-mvp/backend/internal/security/admin_token.go)
* [`IssueAdminToken(secret string, id int64) (string, error)`](file:///E:/Eyewear-mvp/backend/internal/security/admin_token.go#L23-L39): অ্যাডমিন লগইন সফল হলে HS256 অ্যালগরিদমে ২৪ ঘণ্টার মেয়াদি একটি JWT সাইন করে ফেরত দেয় (`Issuer: bongkey-api`, `Audience: bongkey-admin`, `Role: owner`)।
* [`VerifyAdminToken(secret, raw string) (int64, error)`](file:///E:/Eyewear-mvp/backend/internal/security/admin_token.go#L42-L59): ইনকামিং JWT যাচাই করে। সিগনেচার, এক্সপায়ারি, অডিয়েন্স এবং রোল চেক করে অ্যাডমিন আইডি রিটার্ন করে।

#### [`backend/internal/security/order_token.go`](file:///E:/Eyewear-mvp/backend/internal/security/order_token.go)
* (আগের `NewOrderToken` শুধু test-এ ব্যবহার হতো, Audit fix 14-এ মুছে ফেলা হয়েছে। এখন সব guest token `IdempotentOrderToken` থেকে আসে; ডেটাবেসে শুধু SHA-256 হ্যাশ থাকে, কাঁচা টোকেন নয়।)
* [`IdempotentOrderToken(secret, idempotencyKey string) (string, string, error)`](file:///E:/Eyewear-mvp/backend/internal/security/order_token.go#L24-L33): একই Idempotency Key দিয়ে রিট্রাই আসলে HMAC ব্যবহার করে ক্লায়েন্টকে অবিকল একই গেস্ট টোকেন ফেরত দেয়।
* [`MatchesOrderToken(token, storedHash string) bool`](file:///E:/Eyewear-mvp/backend/internal/security/order_token.go#L36-L46): ইনপুট টোকেনটিকে হ্যাশ করে `subtle.ConstantTimeCompare` দিয়ে ডেটাবেস হ্যাশের সাথে তুলনা করে। **এটি ক্রিপ্টোগ্রাফিক টাইমিং সাইড-চ্যানেল অ্যাটাক প্রতিরোধ করে।**

#### [`backend/internal/security/email_proof.go`](file:///E:/Eyewear-mvp/backend/internal/security/email_proof.go)
* [`NewEmailCode() (string, error)`](file:///E:/Eyewear-mvp/backend/internal/security/email_proof.go#L21-L27): ৬-ডিজিটের ক্রিপ্টোগ্রাফিক ওটিপি জেনারেট করে (যেমন: `539201`)।
* [`EmailCodeHash(secret, email, code string)`](file:///E:/Eyewear-mvp/backend/internal/security/email_proof.go#L30-L34): সিক্রেট কি এবং ইমেইল সল্টসহ ওটিপির HMAC হ্যাশ তৈরি করে, যাতে ডিবি লিক হলেও ওটিপি রিভার্স করা না যায়।
* [`EmailKey(secret, kind, value string)`](file:///E:/Eyewear-mvp/backend/internal/security/email_proof.go#L37-L41): রেট লিমিট টেবিলে ব্যবহারের জন্য ইমেইল ও আইপির প্রিফিক্সড HMAC কি তৈরি করে।
* [`IssueEmailProof(secret, email string) (string, error)`](file:///E:/Eyewear-mvp/backend/internal/security/email_proof.go#L44-L55): ওটিপি যাচাই শেষে ১০ মিনিটের মেয়াদি একটি JWT "Email Proof" টোকেন ইস্যু করে।
* [`VerifyEmailProof(secret, raw string) (string, error)`](file:///E:/Eyewear-mvp/backend/internal/security/email_proof.go#L58-L71): প্রুফ টোকেন যাচাই করে ভেরিফায়েড ইমেইল রিটার্ন করে।

---

### ২.৪ মিডলওয়্যার (Middleware)

#### [`backend/internal/middleware/cors.go`](file:///E:/Eyewear-mvp/backend/internal/middleware/cors.go)
* [`CORS() gin.HandlerFunc`](file:///E:/Eyewear-mvp/backend/internal/middleware/cors.go#L10-L23): ব্রাউজার প্রি-ফ্লাইট `OPTIONS` রিকোয়েস্টে `204 No Content` দিয়ে রিটার্ন করে এবং প্রয়োজনীয় CORS হেডার (`Access-Control-Allow-Origin: *`, `Idempotency-Key`, `Authorization`) ইনজেক্ট করে।

#### [`backend/internal/middleware/admin_auth.go`](file:///E:/Eyewear-mvp/backend/internal/middleware/admin_auth.go)
* [`AdminAuth(service auth.Service) gin.HandlerFunc`](file:///E:/Eyewear-mvp/backend/internal/middleware/admin_auth.go#L14-L34):
  * **কী করে:** `Authorization: Bearer <token>` হেডার চেক করে `service.Authenticate` কল করে। টোকেন ইনভ্যালিড হলে `401 Unauthorized` দিয়ে রিকোয়েস্ট `c.AbortWithStatusJSON` করে।
  * ভ্যালিড হলে জিন কন্টেক্সটে `c.Set("admin_id", admin.ID)` সেট করে পরবর্তী হ্যান্ডলারকে এক্সিকিউট করতে দেয় (`c.Next()`)।

---

### ২.৫ অ্যাডমিন ও অথেন্টিকেশন সাবসিস্টেম

#### [`backend/internal/auth/admin.go`](file:///E:/Eyewear-mvp/backend/internal/auth/admin.go)
* [`PostgresRepository`](file:///E:/Eyewear-mvp/backend/internal/auth/admin.go#L34):
  * `ByEmail(ctx, email)`: ইমেইল দিয়ে ওনারের পাসওয়ার্ড হ্যাশ পড়ে আনে।
  * `ByID(ctx, id)`: প্রটেক্টেড রিকোয়েস্টের সময় অ্যাডমিন আইডিটি ডাটাবেসে এখনো বৈধ কিনা নিশ্চিত করে।
* [`Service.Login(ctx, email, password)`](file:///E:/Eyewear-mvp/backend/internal/auth/admin.go#L59-L75): পাসওয়ার্ড ভেরিফিকেশন (bcrypt) করে এবং রোল `owner` হলে `security.IssueAdminToken` দিয়ে সেশন টোকেন ইস্যু করে। ভুল ক্রেডেনশিয়াল হলে নিরাপদ `ErrInvalidCredentials` এরর দেয়।
* [`Service.Authenticate(ctx, token)`](file:///E:/Eyewear-mvp/backend/internal/auth/admin.go#L78-L94): টোকেন ভ্যালিডেশনের পর ডেটাবেসে ওনার একাউন্টটি এখনো কার্যকর আছে কিনা পুনঃপরীক্ষা করে।

---

### ২.৬ ক্যাটালগ সাবসিস্টেম (Categories & Products)

#### [`backend/internal/catalog/categories.go`](file:///E:/Eyewear-mvp/backend/internal/catalog/categories.go) & [`category_repository.go`](file:///E:/Eyewear-mvp/backend/internal/catalog/category_repository.go)
* [`CategoryService`](file:///E:/Eyewear-mvp/backend/internal/catalog/categories.go#L22):
  * `List(ctx)`: সমস্ত ক্যাটাগরি আনে।
  * `Save(ctx, id, category)`: ক্যাটাগরির নাম খালি কিনা চেক করে, নাম থেকে স্লাগ তৈরি করে এবং রিপোজিটরির `Save` কল করে।
  * `Delete(ctx, id)`: আইডি অনুযায়ী ক্যাটাগরি মুছে ফেলে।

#### [`backend/internal/catalog/products.go`](file:///E:/Eyewear-mvp/backend/internal/catalog/products.go) & [`product_repository.go`](file:///E:/Eyewear-mvp/backend/internal/catalog/product_repository.go)
* [`ProductService.Save(ctx, id, input)`](file:///E:/Eyewear-mvp/backend/internal/catalog/products.go#L56-L82):
  * নাম ও প্রাইস বাধ্যতামূলক, স্টক নেগেটিভ হওয়া যাবে না।
  * ওনার যদি কোনো বিদ্যমান প্রোডাক্ট আপডেট করেন, তবে `input.ExpectedStockQuantity` থাকা বাধ্যতামূলক।
* [`PostgresProductRepository.Save(ctx, id, input)`](file:///E:/Eyewear-mvp/backend/internal/catalog/product_repository.go#L57-L95):
  * **Optimistic Concurrency Control (OCC):** আপডেটের সময় SQL কোয়ারিটি লক্ষ্য করুন:
    ```sql
    UPDATE products SET ... WHERE id=$21 AND stock_quantity=$22
    ```
    এখানে `$22` হলো অ্যাডমিনের ফর্ম ওপেন করার সময়ের স্টক। যদি অ্যাডমিন ফর্ম ওপেন রাখার সময়ে লাইভ কোনো কাস্টমার অর্ডার দিয়ে স্টক পরিবর্তন করে ফেলে, তবে `affected == 0` হবে এবং এটি `ErrStaleStock` রিটার্ন করবে! ফলে অ্যাডমিন কোনো লাইভ সেলের ডেটা অসাবধানতাবশত ওভাররাইট করতে পারবেন না।

---

### ২.৭ ওটিপি ও ইমেইল ভেরিফিকেশন সাবসিস্টেম

#### [`backend/internal/emailotp/gmail.go`](file:///E:/Eyewear-mvp/backend/internal/emailotp/gmail.go)
* [`GmailSender.Send(ctx, to, code)`](file:///E:/Eyewear-mvp/backend/internal/emailotp/gmail.go#L23-L74):
  * `smtp.gmail.com:465` এ সরাসরি TLS ডায়াল করে ১০ সেকেন্ডের বাউন্ডেড কন্টেক্সটে মেসেজ পাঠায়।
  * `context.AfterFunc` দিয়ে ক্লায়েন্ট ক্যানসেলেশনের সাথে সাথে TCP সকেট ক্লোজ নিশ্চিত করা হয়।
  * কোনো ক্রেডেনশিয়াল বা এড্রেস এররে লগ করে না।

#### [`backend/internal/emailotp/repository.go`](file:///E:/Eyewear-mvp/backend/internal/emailotp/repository.go)
* [`PostgresStore.Reserve(ctx, emailKey, ipKey, codeHash)`](file:///E:/Eyewear-mvp/backend/internal/emailotp/repository.go#L15-L72):
  * একটি অ্যাটমিক ট্রানজ্যাকশনে ১ দিনের পুরোনো লগ ক্লিন করে।
  * আইপি প্রতি ঘণ্টায় সর্বোচ্চ ১০টি রিকোয়েস্টের বেশি এলে `ErrLimited` (429) দেয়।
  * ইমেইল প্রতি ঘণ্টায় সর্বোচ্চ ৩টি এবং প্রতি ১ মিনিটে ১টির বেশি ওটিপি রিকোয়েস্ট এলে আটকে দেয়।
* [`PostgresStore.Release(ctx, emailKey, codeHash)`](file:///E:/Eyewear-mvp/backend/internal/emailotp/repository.go#L75-L78): জিমেইল ডেলিভারি ব্যর্থ হলে কোডটি সাথে সাথে ইনভ্যালিডেট করে দেয় যাতে অপব্যবহার না হয়।
* [`PostgresStore.Consume(ctx, emailKey, codeHash)`](file:///E:/Eyewear-mvp/backend/internal/emailotp/repository.go#L81-L94): ওটিপি যাচাই করে অ্যাটেম্পট সংখ্যা বাড়ায় (সর্বোচ্চ ৫ বার চেষ্টা করা যাবে), কোড মিলে গেলে `consumed_at = NOW()` সেট করে কোডটি সিঙ্গেল-ইউজ নিশ্চিত করে।

---

### ২.৮ অর্ডার ও চেকআউট সাবসিস্টেম (Core Checkout Engine)

#### [`backend/internal/orders/checkout.go`](file:///E:/Eyewear-mvp/backend/internal/orders/checkout.go)
* [`CheckoutService.Place(ctx, request)`](file:///E:/Eyewear-mvp/backend/internal/orders/checkout.go#L88-L184):
  1. **Idempotency Key ও Item ভ্যালিডেশন:** `ValidIdempotencyKey` চেক করে কি-টি ১৬-১২৮ ক্যারেক্টারের কিনা। প্রতিটি item-এর `LensOption` খালি বা `frameOnlyLens` ("Frame only") না হলে transaction শুরুর আগেই `ErrInvalidCheckout` (400) দেয় — কারণ lens-এর দাম ও prescription review এখনো নেই (Audit fix 1)।
  2. **Deadlock Prevention via Sorted Locking:** কার্টের প্রোডাক্ট আইডিগুলোকে `sort.Ints(productIDs)` দিয়ে ক্রমানুসারে সাজায়, যাতে দুই জন কাস্টমার একই সাথে বিপরীত ক্রমে প্রোডাক্ট কিনলেও রো লক নিয়ে ডাটাবেস ডেডলক না হয়।
  3. **Request Fingerprinting:** `checkoutRequestHash` দিয়ে ক্লায়েন্টের মূল পেলোডের হ্যাশ বের করে।
  4. **Advisory Lock & Distributed Idempotency:** ট্রানজ্যাকশনে ঢুকে `pg_advisory_xact_lock` নেয়। পূর্বের একই Idempotency Key দিয়ে অর্ডার থাকলে এবং পেলোড একই হলে সরাসরি পূর্বের রিসিট রিটার্ন করে। পেলোড ভিন্ন হলে `ErrIdempotencyConflict` (409) দেয়।
  5. **Email Proof Verification:** `security.VerifyEmailProof` দিয়ে যাচাই করে যে ওটিপি ভেরিফিকেশনের ইমেইল এবং অর্ডারের ইমেইল অবিকল এক।
  6. **Atomic Stock Reservation:** `UPDATE products SET stock_quantity = stock_quantity - $2 ... WHERE stock_quantity >= $2` দিয়ে অ্যাটমিকভাবে স্টক হ্রাস করে। স্টক না থাকলে ট্রানজ্যাকশন রোলব্যাক হয়ে `ErrUnavailable` ফেরত দেয়।
  7. **Financial Calculations & Overflow Guard:** ৮০ টাকা ফিক্সড ডেলিভারি ফি সহ সাবটোটাল ও টোটাল হিসাব করে (ইনটিজার ওভারফ্লো চেকসহ)।
  8. **Atomic Insert:** অর্ডার (`ORD-XXXXXX`), অর্ডার আইটেমস, এবং ইনভয়েস (`INV-XXXXXX`) একটিমাত্র ট্রানজ্যাকশনে সেভ করে।
  9. **Deterministic Guest Token:** `security.IdempotentOrderToken` দিয়ে ক্লায়েন্টকে রিট্রাইয়ের ক্ষেত্রেও একই `guest_access_token` ফেরত দেয়।

#### [`backend/internal/orders/access.go`](file:///E:/Eyewear-mvp/backend/internal/orders/access.go)
* [`AccessService.Guest(ctx, id, token)`](file:///E:/Eyewear-mvp/backend/internal/orders/access.go#L48-L63):
  * টোকেনটি ডেটাবেসের `guest_token_hash`-এর সাথে `MatchesOrderToken` দিয়ে কনস্ট্যান্ট-টাইমে চেক করে।
  * যদি না মিলে, তবে টোকেনটি একটি ভ্যালিড `EmailProof` কিনা এবং অর্ডারের ভেরিফায়েড ইমেইলের সাথে মিলে কিনা যাচাই করে।
  * অনুমতি না থাকলে নিরাপদ `ErrNotFound` (404) দেয় (যাতে হ্যাকাররা আইডি এনুমারেট করতে না পারে)।

---

### ২.৯ হ্যান্ডলার তালিকা (Transport Layer)

| হ্যান্ডলার ফাইল | রেসপন্সিবিলিটি ও আচরণ |
| :--- | :--- |
| [`health_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/health_handler.go) | `GET /health` এন্ডপয়েন্টে `{"status":"ok"}` রিটার্ন করে। |
| [`admin_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/admin_handler.go) | অ্যাডমিন লগইন (`POST /api/admin/login`) ও সেশন চেক (`GET /api/admin/me`)। রেসপন্সে `Cache-Control: no-store` নিশ্চিত করে। |
| [`category_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/category_handler.go) | পাবলিক ও অ্যাডমিন ক্যাটাগরি লিস্ট ও CRUD। বডি সাইজ ৬৪KB বাউন্ডেড। |
| [`product_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/product_handler.go) | প্রোডাক্ট ডিসপ্লে (`GET /api/products`, `GET /api/products/:slug`) এবং অ্যাডমিন ম্যানেজমেন্ট। স্টেল স্টকে `409 Conflict` দেয়। |
| [`upload_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/upload_handler.go) | সর্বোচ্চ ৫MB ইমেজ আপলোড নেয়। ম্যাজিক বাইট দিয়ে MIME ভ্যালিডেশন (JPG, PNG, WebP) করে ইউনিক হেক্স নামে `backend/uploads/` এ রাখে। |
| [`email_otp_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/email_otp_handler.go) | ওটিপি সেন্ড (`POST /api/email-otp/request`) ও ওটিপি ভেরিফাই (`POST /api/email-otp/verify`)। ২KB বাউন্ডেড বডি ও পিয়ার আইপি এক্সট্র্যাকশন। |
| [`order_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/order_handler.go) | চেকআউট (`POST /api/orders`)। `Idempotency-Key` পড়ে `CheckoutService.Place` চালায়। |
| [`order_access_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/order_access_handler.go) | গেস্ট ও অ্যাডমিন প্রাইভেট অর্ডার রিডার (`GET /api/orders/:id`)। |
| [`order_list_handler.go`](file:///E:/Eyewear-mvp/backend/internal/handlers/order_list_handler.go) | অ্যাডমিন অর্ডার লিস্ট হ্যান্ডলার (`GET /api/admin/orders`)। |

---

### ২.১০ রাউটিং রেজিস্ট্রি

#### [`backend/internal/routes/routes.go`](file:///E:/Eyewear-mvp/backend/internal/routes/routes.go)
* [`SetupRouter(cfg config.Config) *gin.Engine`](file:///E:/Eyewear-mvp/backend/internal/routes/routes.go#L17-L74):
  * **কী করে:** সমস্ত সার্ভিস ও হ্যান্ডলারকে তাদের প্রয়োজনীয় ডিপেন্ডেন্সি (PostgreSQL কানেকশন পুল, সিক্রেট কি) দিয়ে ইনস্ট্যানশিয়েট করে এবং জিন ইঞ্জিনে রুট হিসেবে তার করে।
  * পাবলিক রাউটস এবং অ্যাডমিন রাউট গ্রুপ (যা `middleware.AdminAuth` দ্বারা আবৃত) আলাদা রাখে।

---

## ৩. গুরুত্বপূর্ণ রিকোয়েস্ট কল ফ্লো (Call Flow Examples)

### ফ্লো ১: অর্ডার প্লেসমেন্ট ও চেকআউট (`POST /api/orders`)
```
[Frontend Client]
       │
       │  POST /api/orders (Header: Idempotency-Key: "uuid-xxxx", Body: customer data + items)
       ▼
routes.SetupRouter ──> routes.go:50
       │
       ▼
handlers.CheckoutHandler.CreateOrder ──> order_handler.go:37
       │ (Body size 64KB bound, Idempotency-Key validation)
       ▼
orders.CheckoutService.Place ──> checkout.go:91
       │
       ├─► LensOption == "" বা "Frame only"?  [না হলে 400, stock ছোঁয় না]
       ├─► sort.Ints(productIDs)  [Deadlock Prevention]
       ├─► checkoutRequestHash()  [Payload Fingerprint]
       ├─► security.IdempotentOrderToken()  [Deterministic Guest Token]
       │
       ▼
orders.PostgresCheckoutStore.InTransaction ──> checkout_repository.go:20
       │
       ├─► BEGIN TX
       ├─► SELECT pg_advisory_xact_lock(keyHash)  [Distributed Idempotency Lock]
       ├─► tx.FindIdempotentOrder(keyHash)  [Duplicate Check]
       │     └─► যদি ইতোমধ্যে অর্ডারটি থাকে ও পেলোড মিলে:
       │           COMMIT TX এবং সরাসরি পূর্বের রিসিট রিটার্ন (200 OK)
       │
       ├─► security.VerifyEmailProof()  [Verified Email Match Check]
       │
       ├─► loop Product IDs:
       │     └─► tx.ReserveProduct(id, qty)  [UPDATE products SET stock = stock - qty WHERE stock >= qty]
       │           └─► যদি স্টক কম থাকে: ROLLBACK TX এবং 409 Conflict রিটার্ন
       │
       ├─► tx.InsertOrder(ORD-XXXXXX)
       ├─► tx.InsertItem(order_items)
       ├─► tx.InsertInvoice(INV-XXXXXX)
       └─► COMMIT TX
       │
       ▼
handlers.CheckoutHandler
       │ (JSON Response: ID, OrderNumber, InvoiceNumber, Total, guest_access_token)
       ▼
[Frontend Client] ── 200 OK
```

### ফ্লো ২: প্রাইভেট অর্ডার ভিউ (`GET /api/orders/:id`)
```
[Client] ── GET /api/orders/ORD-000001 (Header: Authorization: Bearer <TOKEN>)
   │
   ▼
handlers.OrderAccessHandler.GetOrder ──> order_access_handler.go:18
   │
   ▼
orders.AccessService.Guest(ctx, id, token) ──> access.go:48
   │
   ├─► reader.Find(id)  [PostgreSQL থেকে অর্ডার ও guest_token_hash পড়ে]
   │
   ├─► Branch A: security.MatchesOrderToken(token, record.GuestTokenHash)?
   │     └─► True হলে: Authorized! Summary রিটার্ন করে।
   │
   ├─► Branch B: security.VerifyEmailProof(token) == record.CustomerEmail && record.EmailVerified?
   │     └─► True হলে: Authorized! Summary রিটার্ন করে।
   │
   └─► Branch C: কোনোটিই না মিললে:
         └─► রিটার্ন করে ErrNotFound (404 Not Found)। 
             (ক্লায়েন্টকে 403 না দিয়ে 404 দেওয়া হয় যাতে আইডি এনুমারেট করা না যায়)।
```

---

## ৪. ব্যাকএন্ড ইঞ্জিনিয়ার হিসেবে আপনার জন্য ৫টি প্রধান টেকঅ্যাওয়ে (Key Takeaways)

1. **Deadlock Prevention (সর্টেড রো লকিং):** মাল্টিপল আইটেম চেকআউটে আইটেমের আইডিগুলো ক্রমানুসারে সাজিয়ে লক করা হয় (`sort.Ints`)। এটি রিয়েল-ওয়ার্ল্ড হাই-কনকারেন্সি ব্যাকএন্ডের একটি স্ট্যান্ডার্ড প্যাটার্ন।
2. **Distributed Idempotency via Advisory Locks:** একাধিক ব্যাকএন্ড কনটেইনার রান করলেও `pg_advisory_xact_lock` ব্যবহার করায় নেটওয়ার্ক ফ্লিকার বা ডাবল ক্লিকের কারণে কখনোই ডুপ্লিকেট অর্ডার বা একাধিকবার স্টক কাটা সম্ভব নয়।
3. **Optimistic Concurrency Control (OCC):** প্রোডাক্ট এডিটের সময় ডাটাবেসে `stock_quantity` মিলিয়ে আপডেট করা হয়। ফলে লাইভ কাস্টমার সেলসের স্টক অ্যাডমিন ভুলে মুছে ফেলতে পারবেন না।
4. **Zero Information Leakage:** ওটিপি এরর, অর্ডার নট ফাউন্ড, বা ডাটাবেস এররে কখনো কাঁচা ক্রেডেনশিয়াল বা ডাটাবেস এরর মেসেজ ক্লায়েন্টকে দেখানো হয় না।
5. **Context Propagation:** Gin কন্টেক্সট থেকে `c.Request.Context()` নিয়ে সব ডাটাবেস কোয়ারি ও SMTP নেটওয়ার্ক কলে পাস করা হয়েছে। ফলে ক্লায়েন্ট ব্রাউজার ট্যাব কেটে দিলে সাথে সাথে ব্যাকএন্ডের রিসোর্স ড্রেন হওয়া বন্ধ হয়ে যায়।

---

## ৫. Audit fix log (প্রতিটি fix-এর workflow ও শেখার নোট)

প্রতিটি audit fix এখানে একই ছাঁচে লেখা হয়: কী সমস্যা ছিল, request এখন কোন পথে যায়, একটি success ও একটি failure path, আর কোন test সেটি প্রমাণ করে।

### Fix 1 — শুধু "Frame only" বিক্রি (lens option বন্ধ)

* **সমস্যা:** Product page-এ "Blue-cut lens" ও "Prescription lens" বাছা যেত, কিন্তু backend শুধু label সেভ করত, দাম যোগ করত না। Prescription upload-এর input ছিল, কিন্তু file কোথাও যেত না।
* **পরিবর্তন:** Frontend থেকে lens বাছাই ও prescription input সরানো হয়েছে। [`checkout.go`](../backend/internal/orders/checkout.go) এ `frameOnlyLens` rule যোগ হয়েছে। পুরনো cart-এর lens item [`cart.tsx`](../frontend/src/lib/cart.tsx) এর `frameOnlyItems` "Frame only"-তে merge করে।
* **Success flow:** Product page `addItem(product)` → cart-এ `lensOption: "Frame only"` → checkout `POST /api/orders` body-তে `lens_option: "Frame only"` → `CheckoutHandler.CreateOrder` → `CheckoutService.Place` validation pass → transaction → `InsertItem` lens snapshot সেভ → 200।
* **Failure flow:** কেউ API-তে সরাসরি `lens_option: "Prescription lens"` পাঠালে `Place`-এর প্রথম validation loop `ErrInvalidCheckout` দেয় → handler 400 `invalid order data`। Transaction শুরুই হয় না, তাই stock কমে না।
* **Middleware না service?** এটা business rule, তাই service-এ। Handler শুধু JSON parse করে; middleware-এ দাম/stock rule রাখা নিষেধ।
* **Test:** `TestCheckoutRejectsUnpricedLensOption` — তিনটি অবৈধ মান reject হয় এবং `store.started == false`; "Frame only" সফল হয়।
* **নিজে বলে দেখো:** কেন frontend থেকে option সরানোই যথেষ্ট না? (উত্তর: API সরাসরি call করা যায়; backend authoritative।)

### Fix 2 — Owner ও guest order-এর item দেখতে পান

* **সমস্যা:** `GET /api/admin/orders/:id` ও `GET /api/orders/:id` শুধু নাম, ঠিকানা, total দিত। কোন frame কয়টা — এটা কোনো API দিত না, তাই owner pack করতে পারতেন না।
* **পরিবর্তন:** [`access.go`](../backend/internal/orders/access.go) এ `Summary.Items []Item`; [`repository.go`](../backend/internal/orders/repository.go) এর `PostgresReader.Find` order row পড়ার পর `items()` দিয়ে `order_items` snapshot পড়ে; [`order_access_handler.go`](../backend/internal/handlers/order_access_handler.go) এর `respondWithOrder` JSON-এ `items` দেয়। Admin orders page-এ "View items" button, success page-এ item list।
* **Success flow (admin):** "View items" click → `fetchAdminOrder(id)` → `GET /api/admin/orders/:id` → `CORS` → `AdminAuth` (token + current owner role, `c.Next()`) → `OrderAccessHandler.GetAdminOrder` → `AccessService.Admin` → `PostgresReader.Find` (order + invoice JOIN, তারপর `items` query) → `respondWithOrder` 200 `{..., items: [...]}` → page-এ `details[order.id]` state।
* **Failure flow:** Token ছাড়া call করলে `AdminAuth` `AbortWithStatusJSON(401)` করে return করে — handler চলেই না। Guest-এর ভুল token হলে `AccessService.Guest` `ErrNotFound` → 404, item বা ঠিকানা কিছুই যায় না।
* **কেন snapshot table থেকে:** `order_items.product_name/unit_price` checkout-এর সময়ের মান; পরে product-এর দাম বা নাম বদলালেও পুরনো order বদলায় না।
* **Test:** `TestGuestOrderResponse` এখন item name-ও private data হিসেবে check করে: শুধু সঠিক token-এ আসে।
* **নিজে বলে দেখো:** Item কেন `products` table JOIN না করে `order_items` থেকে পড়া হয়?

### Fix 3 — Admin login-এ rate limit ও timing-safe check

* **সমস্যা:** `POST /api/admin/login` যত খুশি password আন্দাজ নিত; body size limit ছিল না; ভুল email-এ bcrypt না চলায় উত্তর দ্রুত আসত, ফলে owner-এর email ধরা যেত।
* **পরিবর্তন:** [`admin.go`](../backend/internal/auth/admin.go) এ `LoginLimiter` interface ও `PostgresLoginLimiter` (migration [`010_admin_login_limits.sql`](../backend/migrations/010_admin_login_limits.sql))। প্রতি IP প্রতি ১৫ মিনিটে ১০টি চেষ্টা। অজানা email-এ `dummyPasswordHash()` দিয়ে একই bcrypt কাজ হয়। [`admin_handler.go`](../backend/internal/handlers/admin_handler.go) ৪KB body limit আর `RemoteAddr` থেকে IP নেয়।
* **Success flow:** Login form → `POST /api/admin/login` → `CORS` → `AdminHandler.Login` (MaxBytesReader, JSON bind, `net.SplitHostPort`) → `auth.Service.Login(ctx, email, password, ip)` → `Limiter.Count` (একটি atomic `INSERT ... ON CONFLICT DO UPDATE ... RETURNING`) → attempts ≤ 10 → `ByEmail` → `bcrypt.CompareHashAndPassword` → `security.IssueAdminToken` → 200 `{token}`।
* **Failure flow:** ১১তম চেষ্টায় `Count` 11 ফেরত দেয় → service `ErrLoginLimited` → handler 429। Password check আর চলেই না। Limiter কনফিগার না থাকলে service error দেয় (fail closed) → 503।
* **কেন PostgreSQL, mutex না:** Process-local counter একাধিক server instance-এ ভাগ হয় না; database row সব instance-এর জন্য একটাই budget।
* **কেন IP hash:** `security.EmailKey(secret, "admin-login-ip", ip)` HMAC রাখে; table dump থেকে কাঁচা IP পাওয়া যায় না।
* **Test:** `TestLoginRateLimitStopsPasswordGuessing` (১০ ভুলের পর সঠিক password-ও 429, অন্য IP চলে, limiter ছাড়া fail closed); `TestPostgresLoginLimiterCountsConcurrentAttempts` আসল PostgreSQL-এ ১০টি একসাথে চেষ্টা গুনে ১১তমে 11 পায়।
* **নিজে বলে দেখো:** কেন check আর increment আলাদা দুই query হলে race হতো?

### Fix 4 ও 5 — Checkout-এ বর্তমান দাম, আর নষ্ট cart-এ site বন্ধ না হওয়া

* **সমস্যা (4):** Cart পুরো product দাম সহ `localStorage`-এ রাখত। পরে owner দাম বদলালে checkout পুরনো দাম দেখাত, অথচ server নতুন দাম নিত।
* **সমস্যা (5):** `JSON.parse` ভাঙা data পেলে error ছুড়ত; `CartProvider` layout-এ থাকায় পুরো site সাদা হয়ে যেত।
* **পরিবর্তন:** [`cart.tsx`](../frontend/src/lib/cart.tsx) এ `loadSavedItems` (try/catch + shape check), `ready` flag (load শেষ হওয়ার আগে save করে না), আর `refreshProducts(current)`। [`checkout/page.tsx`](../frontend/src/app/checkout/page.tsx) cart load হলে `fetchProducts()` ডাকে, দাম মিলিয়ে cart আপডেট করে, পরিবর্তন থাকলে notice দেখায়; দাম নিশ্চিত না হওয়া পর্যন্ত "Place Order" বন্ধ।
* **Success flow:** Checkout খোলা → `CartProvider` effect `loadSavedItems()` → `ready = true` → checkout effect `checkCurrentPrices()` → `GET /api/products` → `ProductHandler.ListProducts` → `ProductService.List` → `PostgresProductRepository.List` → 200 → `refreshProducts` নতুন দাম বসায় → summary-তে আসল total → submit → `POST /api/orders` (server আবার নিজেই দাম হিসাব করে)।
* **Failure flow:** `GET /api/products` fail করলে `priceCheck = "failed"`, "Try again" button দেখায়, submit বন্ধ থাকে। Product বন্ধ বা stock শূন্য হলে item cart থেকে সরে যায় আর customer-কে কারণ দেখায়।
* **মনে রাখো:** এটা শুধু দেখানোর সঠিকতা। দামের চূড়ান্ত সিদ্ধান্ত এখনো `CheckoutService.Place`-এ `ReserveProduct` থেকে আসা server দাম।
* **React শেখা:** Child-এর `useEffect` parent-এর আগে চলে, তাই `ready` flag ছাড়া checkout খালি cart-এ দাম মেলাত।
* **Verification:** TypeScript ও production build পাস। Frontend-এ test runner নেই, তাই এই logic-এর automated test নেই; browser-এ হাতে দেখা বাকি।

### Fix 6 — HTTP timeout, graceful shutdown ও database pool

* **সমস্যা:** `router.Run()` Go-র default server চালাত — কোনো read/write timeout নেই, তাই খুব ধীরে data পাঠানো কিছু connection (slowloris) server আটকে রাখতে পারত। Ctrl+C বা deploy restart-এ চলমান checkout মাঝপথে কেটে যেত। Database pool-এর সীমা ছিল না।
* **পরিবর্তন:** [`server.go`](../backend/cmd/server/server.go) এ `newHTTPServer` (ReadHeader 5s, Read/Write 30s, Idle 60s) আর `runHTTP`। `signal.NotifyContext` Ctrl+C/SIGTERM পেলে `server.Shutdown` নতুন connection বন্ধ করে, চলমান request ১৫ সেকেন্ড পর্যন্ত শেষ হতে দেয়। [`database.go`](../backend/internal/database/database.go) এর `Connect` এ pool সীমা (২০ open, ১০ idle) আর ১০ সেকেন্ডের `PingContext`।
* **Startup flow:** `main` → `config.Load` → `serve` → `database.Connect` → `MigrateAndSeed` → `routes.SetupRouter` (শুধু route **register** করে; handler এখন চলে না) → `runHTTP` → goroutine-এ `ListenAndServe` → `select` অপেক্ষা করে।
* **Shutdown flow:** Ctrl+C → `ctx.Done()` → `server.Shutdown(15s)` → চলমান request শেষ → `ListenAndServe` `http.ErrServerClosed` দেয় → `runHTTP` nil → `serve`-এর `defer database.DB.Close()` — pool শেষে বন্ধ হয়, যাতে drain হওয়া request তখনো database পায়।
* **Failure flow:** Port আগে থেকেই ব্যস্ত হলে `ListenAndServe` সাথে সাথে error দেয় → `select` প্রথম case → `listen on :8080: ...` → `main` `log.Fatalf`।
* **Goroutine-এর মালিকানা:** `runHTTP` goroutine-টির মালিক; buffered channel (size 1) তাই goroutine কখনো আটকে থাকে না; `Shutdown`-এর পর তার result পড়ে নিশ্চিত করা হয় যে সেটি শেষ হয়েছে।
* **Test:** `TestRunHTTPDrainsInFlightRequestOnShutdown` — একটি request চলার সময় shutdown দিলে request 204 পায় আর `runHTTP` nil ফেরত দেয়। `go test -race` (UCRT64 GCC) পাস।

### Fix 7 — Upload করা ছবির URL host ছাড়া (relative path)

* **সমস্যা:** Upload শেষে server `http://<Host>/uploads/...` বানিয়ে দিত আর সেটাই product-এ সেভ হতো। Local-এ upload করলে DB-তে `localhost:8080` থেকে যেত, deploy-এর পর সব ছবি ভাঙত। Code `X-Forwarded-Proto` header-ও বিশ্বাস করত, যা PROXY-01 নীতির উল্টো।
* **পরিবর্তন:** [`upload_handler.go`](../backend/internal/handlers/upload_handler.go) এখন শুধু `/uploads/<folder>/<file>` দেয়; `absoluteUploadURL` মুছে ফেলা হয়েছে। Frontend-এ তিনটি একই রকম `toImageSrc` ([`ProductForm.tsx`](../frontend/src/components/ProductForm.tsx), `ProductVisual.tsx`, `TryOnModal.tsx`) একটি `assetUrl` ([`api.ts`](../frontend/src/lib/api.ts)) দিয়ে বদলানো — এটা path-এর আগে `NEXT_PUBLIC_API_URL` বসায়, পুরনো absolute URL-ও চালিয়ে যায়।
* **Success flow:** Admin ছবি বাছে → `adminUpload` → `POST /api/admin/uploads/product-image` → `CORS` → `AdminAuth` → `UploadHandler.ProductImage` → `uploadImage` (5MB limit, extension ও magic-byte check, random নাম) → 200 `{url: "/uploads/products/..."}` → form-এ সেভ → storefront `assetUrl` দিয়ে দেখায়।
* **Failure flow:** `.png` নামে text file দিলে `http.DetectContentType` PNG চেনে না → 400; disk-এ কিছু লেখা হয় না।
* **Test:** `TestProductImageUploadReturnsHostFreePath` — নকল Host ও `X-Forwarded-Proto` থাকলেও URL host-free, file disk-এ আছে; নকল image 400।

### Fix 8 — যাচাইহীন `prescription_file_url` সরানো

* **সমস্যা:** Checkout request যেকোনো string `prescription_file_url` হিসেবে নিত আর order-এ সেভ করত। কোনো যাচাই নেই, কোথাও দেখানোও হতো না। পরে admin page-এ দেখালে বাইরের phishing link বা `javascript:` URL ঢুকে যেত।
* **পরিবর্তন:** [`order_handler.go`](../backend/internal/handlers/order_handler.go), [`checkout.go`](../backend/internal/orders/checkout.go) আর [`checkout_repository.go`](../backend/internal/orders/checkout_repository.go) থেকে field সরানো। Gin JSON bind অজানা field উপেক্ষা করে, তাই পুরনো client পাঠালেও কিছু সেভ হয় না। DB column থাকছে (default `''`), কারণ column মুছে ফেলা destructive migration।
* **Idempotency সতর্কতা:** `checkoutRequestHash` এ field-টি খালি রেখে দেওয়া হয়েছে, যাতে আগে accepted order-এর retry-র fingerprint একই থাকে; না হলে retry 409 পেত।
* **Flow:** `POST /api/orders` → `CheckoutHandler.CreateOrder` (prescription field আর bind হয় না) → `CheckoutService.Place` → `InsertOrder` (column বাদ দিয়ে ১৫টি parameter) → 200।
* **R1-এ সঠিক পথ:** Prescription private storage-এ upload, file type/size/signature check, প্রতি order authorization — তারপর আলাদা story।
* **Test:** `TestCheckoutRequestHashMatchesPrePrescriptionRemoval` পুরনো fingerprint মেলায়; আসল PostgreSQL-এ `TestCheckoutOrderPostgres`, `TestConcurrentIdempotentCheckoutPostgres` সহ checkout integration test পাস।

### Fix 9 — ব্যবহার না হওয়া demo product data মোছা

* **সমস্যা:** [`products.ts`](../frontend/src/lib/products.ts) এ ৪০০+ লাইনের hardcoded `products` array আর `getProduct`, `featuredProducts`, `relatedProducts` ছিল। পুরো repo grep করে দেখা গেছে কেউ import করে না; শুধু `Product` type আর `formatPrice` ব্যবহার হয়।
* **ঝুঁকি:** ভুলে কেউ import করলে API fail হলেও নকল দাম ও stock আসল দোকানে দেখাত — PRD এটা নিষেধ করে।
* **পরিবর্তন:** Array ও তিনটি function মুছে ফেলা; file এখন ৩২ লাইন। সব product এখন শুধু `GET /api/products` থেকে আসে।
* **Verification:** TypeScript ও production build পাস (কোনো import ভাঙেনি)।

### Fix 10 — Admin order list-এ pagination (keyset)

* **সমস্যা:** `ListAdmin` সব order একবারে পাঠাত; কয়েক হাজার order হলে admin page ধীর হতো।
* **পরিবর্তন:** [`repository.go`](../backend/internal/orders/repository.go) এ `ListAdmin(ctx, beforeID)` প্রতি page-এ `AdminOrderPageSize` (৫০)টি order দেয়: `WHERE ($1 = 0 OR id < $1) ORDER BY id DESC LIMIT $2`। [`order_list_handler.go`](../backend/internal/handlers/order_list_handler.go) `?before=<id>` পড়ে ও যাচাই করে। Admin page-এ "Load older orders" button।
* **কেন OFFSET না:** নতুন order আসলে OFFSET সব page এক ঘর সরিয়ে দেয় — একই order দুবার বা একটিও না দেখা যায়। Primary key দিয়ে "এর চেয়ে পুরনো" চাইলে page স্থির থাকে আর index ব্যবহার হয়।
* **Success flow:** Page খোলা → `loadOrders()` → `GET /api/admin/orders` → `AdminAuth` → `ListOrders` (beforeID = 0) → `ListAdmin` → ৫০টি → ৫০টি এলে button দেখায় → click → `GET /api/admin/orders?before=<শেষ id>`।
* **Failure flow:** `?before=abc`, `0` বা ঋণাত্মক হলে handler database ছোঁয়ার আগেই 400।
* **Test:** `TestListOrdersRejectsInvalidCursor`; আসল PostgreSQL-এ `TestCheckoutOrderPostgres` নতুন order প্রথম page-এর শুরুতে আর `before=<id>` page-এ পুরনোগুলোই আছে কিনা দেখে (Fix 2-এর item snapshot-ও এখানে DB-তে যাচাই হয়)।

### Fix 11 — Order খোঁজার query index ব্যবহার করে

* **সমস্যা:** `WHERE o.id::text = $1 OR o.order_number = $1` — `id`-কে text বানালে primary key index কাজ করে না, প্রতিবার পুরো `orders` table scan হতো।
* **পরিবর্তন:** [`repository.go`](../backend/internal/orders/repository.go) এর `PostgresReader.Find` আগে দেখে মানটা সংখ্যা কিনা (`strconv.Atoi`)। সংখ্যা হলে `o.id = $1`, না হলে `o.order_number = $1`। দুটোই index ব্যবহার করে। WHERE অংশ দুটি নির্দিষ্ট constant; user-এর text কখনো SQL-এ জোড়া হয় না, মান সবসময় `$1` parameter।
* **Flow:** `GET /api/orders/12` → `GetOrder` → `AccessService.Guest` → `Find("12")` → `o.id = 12`। `GET /api/orders/ORD-000012` → `o.order_number = 'ORD-000012'`।
* **Failure flow:** `0` বা ঋণাত্মক id হলে query না চালিয়েই `ErrNotFound` → 404।
* **Test:** PostgreSQL `TestCheckoutOrderPostgres` এখন id ও order number দুইভাবেই একই order পায়, আর `"0"` 404 দেয়।

### Fix 12 — Cancel ও checkout একই ক্রমে lock নেয় (deadlock প্রতিরোধ)

* **সমস্যা:** Checkout product row-গুলো id-র ছোট থেকে বড় ক্রমে lock করে (`sort.Ints`)। কিন্তু cancel-এর `RestoreStock` একটি `UPDATE ... FROM` দিয়ে যে ক্রমে PostgreSQL চায় সেই ক্রমে lock নিত। একই দুটি product-এ একসাথে checkout (A তারপর B) আর cancel (B তারপর A) হলে deadlock — PostgreSQL একটি transaction বাতিল করে 500 দিত।
* **পরিবর্তন:** [`fulfillment_repository.go`](../backend/internal/orders/fulfillment_repository.go) এর `RestoreStock` আগে `SELECT id FROM products WHERE id IN (...) ORDER BY id FOR UPDATE` দিয়ে একই ক্রমে lock নেয়, তারপর stock ফেরত দেয়।
* **Flow:** Admin "Cancel order" → `PATCH /api/admin/orders/:id/status` → `AdminAuth` → `OrderFulfillmentHandler.ChangeStatus` → `FulfillmentService.ChangeStatus` → `InTransaction` → `LockOrder` (`FOR UPDATE`) → transition check → `RestoreStock` (sorted lock → update) → `UpdateStatus` → COMMIT।
* **নিয়মটা মনে রাখো:** একাধিক row lock করতে হলে সব জায়গায় একই ক্রম — তাহলে কেউ একে অপরের জন্য গোল হয়ে অপেক্ষা করে না।
* **Test:** আসল PostgreSQL-এ `TestConcurrentCancellationPostgres` সহ সব order integration test পাস। Deadlock-এর জন্য আলাদা নির্ভরযোগ্য test নেই, কারণ timing ছাড়া deadlock তৈরি করা কঠিন; এটা জানা সীমাবদ্ধতা।

### Fix 13 — একটি মাত্র `Slugify`

* **সমস্যা:** [`products.go`](../backend/internal/catalog/products.go) আর [`database.go`](../backend/internal/database/database.go) এ হুবহু একই `slugify` ছিল। একটায় নিয়ম বদলালে seed আর admin-তৈরি product আলাদা slug পেত।
* **পরিবর্তন:** `catalog.Slugify` export করা হয়েছে; seed code (`seedCategories`, `seedProducts`) সেটাই ডাকে। Dependency দিক: `database` → `catalog` (catalog database-কে import করে না, তাই cycle নেই)।
* **Verification:** `go vet` ও পুরো backend test পাস।

### Fix 14 — Dead code ও পুরনো doc মোছা

* **মোছা হয়েছে:** `security.NewOrderToken` (production code-এ কোথাও ব্যবহার হতো না; test fixture এখন `IdempotentOrderToken` ব্যবহার করে), খালি `backend/internal/models/placeholder.go`, আর পুরোপুরি পুরনো `docs/FOUNDATION.md`।
* **Coverage রাখা:** `TestOrderToken`-এর ভুল token ও খালি legacy hash-এর negative check `TestIdempotentOrderToken`-এ সরানো হয়েছে।
* **এই doc-এর উপরের অংশ:** Security layer diagram, `order_token.go` ও seed `Slugify` লাইন নতুন code অনুযায়ী ঠিক করা হয়েছে।
* **Verification:** `go vet` ও পুরো backend test পাস।

### Migration নম্বর নোট

Main checkout-এ commit না করা কাজে `007_delivery_settings.sql`, `008_product_variants.sql` ও `009_delivery_zones.sql` আছে। নম্বরের সংঘাত এড়াতে login limit migration-টির নাম `010_admin_login_limits.sql`। দুই কাজ merge করার সময় [`database.go`](../backend/internal/database/database.go) এর migration তালিকা ০০৭ → ০১০ ক্রমে মিলিয়ে নিতে হবে।

## ৬. Sprint S0 — main-এর কাজ আর audit fix এক করা

Main checkout-এ commit না করা delivery settings ও zones, product variants, PDF invoice আর admin order detail page-এর কাজ একটা branch-এ নেওয়া হয়েছে, তারপর audit fix-গুলোর সাথে merge করা হয়েছে। দুই দিকেই একই সমস্যার সমাধান থাকলে যেটা বেশি নিরাপদ বা বেশি সম্পূর্ণ, সেটা রাখা হয়েছে:

* **Frame-only নিয়ম (Fix 1):** Main-এর version রাখা হয়েছে — `CheckoutService.Place` transaction-এর ভেতরে, idempotency lookup-এর **পরে** lens যাচাই করে। ফলে আগে accept হওয়া lens order-এর retry এখনো তার receipt পায়; নতুন lens order 400। আমার আগের early check (transaction-এর আগে) সরানো হয়েছে, আর duplicate test মুছে main-এর `TestNewCheckoutRejectsUnpricedLenses`-এ "frame only" (ভুল case) যোগ হয়েছে।
* **Order item দেখা (Fix 2):** Main-এর design রাখা হয়েছে — `AccessService.Admin` → `AdminDetail` (`Reader.Items`) admin order detail page (`/admin/orders/[id]`)-এ item, SKU, payment দেখায়। Guest JSON-এ item থাকে না (main-এর test এটা নিশ্চিত করে); guest item দেখে PDF invoice-এ। তাই list-এর ভেতরের "View items" আর success page-এর item তালিকা সরানো হয়েছে।
* **Order lookup (Fix 11):** Main-এর বাড়তি column (`delivery_note`, `payment_method`, `payment_status`, `created_at`, `issued_at`) আমার typed-key query (`findOrderSQL`)-তে যোগ হয়েছে।
* **Prescription field (Fix 8):** Main-এর `ExpectedDeliveryFee` রাখা হয়েছে, `PrescriptionFileURL` সরানো হয়েছে। আগে যে order prescription text দিয়ে accept হয়েছিল, তার retry এখন fingerprint না মেলায় 409 পেতে পারে — R0-তে এমন আসল order নেই বলে এটা গ্রহণযোগ্য।
* **Cart ও checkout (Fix 4–5):** Main-এর district dropdown ও delivery settings-এর সাথে আমার `loadSavedItems`, `ready` flag আর `refreshProducts` একসাথে আছে। `checkCurrentPrices` এখন product আর delivery settings দুটোই একসাথে আনে; "Place Order" চালু হয় শুধু district বাছা হলে **এবং** দাম নিশ্চিত হলে।
* **Migration ক্রম:** `002…006`, `007_delivery_settings`, `008_product_variants`, `009_delivery_zones`, `010_admin_login_limits`।

**খোলা বিষয় (PRD-র সাথে অমিল):** Main-এর delivery zone এখন "Dhaka বনাম বাকি সব" দুটি নির্দিষ্ট fee। PRD section 7 বলে hardcoded Dhaka/outside-Dhaka নিয়ম থাকবে না, admin zone বানাবেন ও district map করবেন। R0-তে এটা চলতে পারে, কিন্তু R1-এর আগে S4-এ আসল zone model লাগবে।

**Verification:** `go vet`, পুরো `go test`, `go test -race` (UCRT64 GCC), TypeScript আর production build পাস। PostgreSQL integration test বাকি — local database-এ `009` ও `010` migration এখনো চালানো হয়নি।
