package main

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	redisStore "github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"golang.org/x/crypto/bcrypt"
	"log"
	"recipe-api-copy/handlers"
	"time"
)

var authHandler *handlers.AuthHandler
var recipesHandler *handlers.RecipesHandler

func init() {
	users := map[string]string{
		"admin": "pwd123",
		"packt": "pwd123",
		"ldeng": "pwd123",
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://admin:password@localhost:27017/test?authSource=admin"))
	if err = client.Ping(context.TODO(),
		readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")

	collectionUsers := client.Database("demo").Collection("users")

	count, err := collectionUsers.CountDocuments(ctx, bson.D{})

	if count == 0 {
		for username, password := range users {
			bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
			if err != nil {
				panic(err)
			}

			log.Println("Inserting user: " + username)

			collectionUsers.InsertOne(ctx, bson.M{
				"username": username,
				"password": string(bytes),
			})
		}
	} else {
		log.Println("Found users in Db. Skip inserting.")
	}

	collectionRecipes := client.Database("demo").Collection("recipes")

	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	status := redisClient.Ping()
	fmt.Println(status)

	recipesHandler = handlers.NewRecipesHandler(ctx, collectionRecipes, redisClient)
	//authHandler = &handlers.AuthHandler{}

	collectionUsers = client.Database("demo").Collection("users")
	authHandler = handlers.NewAuthHandler(ctx, collectionUsers)
}

func main() {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://admin:password@localhost:27017/test?authSource=admin"))
	if err = client.Ping(context.TODO(),
		readpref.Primary()); err != nil {
		log.Fatal(err)
	}

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3001"},
		AllowMethods:     []string{"GET", "OPTIONS"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	store, _ := redisStore.NewStore(10, "tcp", "localhost:6379", "", []byte("secret"))
	router.Use(sessions.Sessions("recipes_api", store))

	authorized := router.Group("/")

	router.POST("/signin", authHandler.SignInHandler)
	authorized.GET("/recipes", recipesHandler.ListRecipesHandler)
	router.POST("/signout", authHandler.SignOutHandler)

	authorized.Use(authHandler.AuthMiddleware())
	{
		authorized.POST("/recipes", recipesHandler.NewRecipeHandler)
		authorized.PUT("/recipes/:id", recipesHandler.UpdateRecipeHandler)
		authorized.DELETE("/recipes/:id", recipesHandler.DeleteRecipeHandler)
		authorized.GET("/recipes/:id", recipesHandler.GetOneRecipeHandler)
	}

	router.Run()
}
