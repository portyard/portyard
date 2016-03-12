package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

// User data struct
type User struct {
	ID        int        `sql:"AUTO_INCREMENT" json:"id"`
	UserName  string     `json:"user_name"`
	Type      string     `json:"type"`
	Name      string     `sql:"size:255" json:"name"` // Default size for string is 255, you could reset it with this tag
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`

	Emails   []Email   `json:"emails"`                                   // One-To-Many relationship (has many)
	Projects []Project `json:"projects" gorm:"many2many:user_projects;"` // Many-To-Many relationship (has many)
}

// Email data
type Email struct {
	ID         int    `json:"email_id"`
	UserID     int    `sql:"index" json:"user_id"`                        // Foreign key (belongs to), tag `index` will create index for this field when using AutoMigrate
	Email      string `sql:"type:varchar(100);unique_index" json:"email"` // Set field's sql type, tag `unique_index` will create unique index
	Subscribed bool   `json:"subscribed"`
}

// Project Data
type Project struct {
	ID          int         `json:"project_id"`
	Author      string      `sql:"index" json:"author"`
	ProjectName string      `sql:"not null" json:"project_name"` // Create index with name, and will create combined index if find other fields defined same name
	CreatedAt   time.Time   `json:"created_at"`
	Components  []Component `json:"components,omitempty"` // One-To-Many relationship (has many)
}

// Component data
type Component struct {
	ID            int       `json:"component_id"`
	ProjectID     int       `sql:"index" json:"project_id"`
	ComponentName string    `json:"component_name"`
	ProjectName   string    `json:"project_name"`
	Releases      []Release `json:"releases"`
}

// Release Data
type Release struct {
	CreatedAt string `json:"created_at"`
	URL       string `json:"url"`
	Version   string `json:"version"`
}

// UserProjects data
type UserProjects struct {
	UserID    int `json:"user_id"`
	ProjectID int `json:"project_id"`
}

// Members data struct
type Members struct {
	Members []struct {
		Key string `json:"key"`
	} `json:"members"`
}

// StatusMessage json
type StatusMessage struct {
	Error   string `json:"error,omitempty"`
	Success string `json:"success,omitempty"`
}

// Database
var DB *gorm.DB

func init() {
	db, err := gorm.Open("postgres", "user=postgres password=repo-api sslmode=disable host=192.168.153.128 port=5432")
	if err != nil {
		panic(fmt.Sprintf("Got error when connect database, the error is '%v'", err))
	}

	pingErr := db.DB().Ping()
	if pingErr != nil {
		log.Fatalln(pingErr)
	}

	db.CreateTable(&User{})
	db.CreateTable(&Email{})
	db.CreateTable(&Project{})
	db.CreateTable(&Component{})
	db.CreateTable(&Release{})
	db.AutoMigrate(&User{}, &Email{}, &Project{}, &Component{}, &Release{})

	DB = &db
}

func main() {
	r := gin.Default()
	v1 := r.Group("v1")
	{
		v1.GET("/users", getUsers)
		v1.GET("/user/:username/projects", getUserProjects)
		v1.GET("/user/:username", getUser)
		v1.POST("/user", postUser)
		v1.DELETE("/users/:id", deleteUser)
		v1.GET("/projects", getProjects)
		v1.GET("/project/:name", getProject)
		v1.PUT("/project/:project_name/members/create", updateUserProject)
		v1.POST("/project", postProject)
		v1.DELETE("/projects/:name", deleteUser)
		v1.GET("/components", getComponents)
		v1.GET("/component/:name", getComponent)
		v1.POST("/component/create", postComponent)
		v1.PUT("/component/:name", updateComponents)
		v1.DELETE("/components/:name", deleteComponent)
	}
	r.Run(":8080")
}

func getUsers(c *gin.Context) {

	users := []User{}
	DB.Find(&users)

	for i := range users {
		DB.Model(users[i]).Related(&users[i].Emails)
	}

	c.JSON(200, users)
}

func getUser(c *gin.Context) {

	userName := c.Params.ByName("username")
	user := User{}

	if DB.Where("user_name = ?", userName).First(&user).RecordNotFound() {
		content := StatusMessage{Error: "Project with username " + userName + " not found"}
		c.JSON(404, content)
	} else {
		c.JSON(200, &user)
	}
}
func getProject(c *gin.Context) {

	projectName := c.Params.ByName("name")
	project := Project{}
	component := Component{}
	// release := Release{}

	if DB.Where("project_name = ?", projectName).First(&project).RecordNotFound() {
		content := StatusMessage{Error: "Project with project name " + projectName + " not found"}
		c.JSON(404, content)
	} else if DB.Where("project_id = ?", project.ID).Find(&component).RecordNotFound() {
		content := StatusMessage{Error: "Componets for " + projectName + " not found"}
		c.JSON(404, content)
	} else {
		DB.Model(project).Related(&project.Components)
		DB.Model(component).Related(&component.Releases)
		fmt.Println(component)
		c.JSON(200, &project)
	}
}

func getUserProjects(c *gin.Context) {

	userName := c.Params.ByName("username")

	user := User{}
	list := []Project{}
	projects := Project{}
	userProjects := []UserProjects{}

	if DB.Where("user_name = ?", userName).First(&user).RecordNotFound() {
		content := gin.H{"error": "user with username" + userName + " not found"}
		c.JSON(404, content)
	} else if DB.Where("user_id = ?", user.ID).Find(&userProjects).RecordNotFound() {
		content := StatusMessage{Error: "User has no projects"}
		c.JSON(400, content)
	} else {
		for _, obj := range userProjects {
			DB.Where("id = ?", obj.ProjectID).Find(&projects)
			list = append(list, projects)
			projects = Project{}
		}
		c.JSON(200, &list)
	}

}

func getProjects(c *gin.Context) {

	projects := []Project{}
	DB.Find(&projects)

	for i := range projects {
		DB.Model(projects[i]).Related(&projects[i].Components)
	}

	c.JSON(200, &projects)
}

func postUser(c *gin.Context) {

	userData := User{}

	body, _ := ioutil.ReadAll(c.Request.Body)

	err := json.Unmarshal(body, &userData)
	if err != nil {
		panic(err)
	}

	user := User{
		Name:     userData.Name,
		UserName: userData.UserName,
		Type:     userData.Type,
		Emails:   userData.Emails,
		Active:   userData.Active,
	}

	DB.Create(&user)
}
func updateUserProject(c *gin.Context) {
	userData := Members{}

	body, _ := ioutil.ReadAll(c.Request.Body)

	err := json.Unmarshal(body, &userData)
	if err != nil {
		panic(err)
	}

	projectName := c.Params.ByName("project_name")

	projectExists, pID := checkProject(projectName)
	userExists, uIDs := checkUsers(userData)

	if projectExists && userExists {
		status, message := updateUser(userData, pID, uIDs)
		c.JSON(status, message)
	} else if !userExists {
		c.JSON(400, StatusMessage{Error: "Request body invalid, a user supplied is already a member of that project or a user supplied does not exist"})
	} else {
		c.JSON(404, StatusMessage{Error: "Project does not exist"})
	}

}

func updateUser(users Members, pID int, uIDs []int) (int, StatusMessage) {

	var results error

	for _, user := range uIDs {
		newAssociation := UserProjects{
			UserID:    user,
			ProjectID: pID,
		}
		results = DB.Create(newAssociation).Error
	}

	if results != nil {
		return 400, StatusMessage{Error: "Request body invalid, a user supplied is already a member of that project or a user supplied does not exist"}
	}

	return 200, StatusMessage{Success: "Added to project"}
}

func checkProject(p string) (bool, int) {

	project := Project{}
	projects := false

	if DB.Where("project_name = ?", p).First(&project).RecordNotFound() {
		projects = false
	} else {
		projects = true

	}
	return projects, project.ID

}

func checkUsers(u Members) (bool, []int) {

	user := User{}
	userExists := true
	var userList []int

	for _, obj := range u.Members {

		if DB.Where("user_name = ?", obj.Key).First(&user).RecordNotFound() {
			userExists = false
			fmt.Println(StatusMessage{Error: "user with username " + obj.Key + " not found"})
		}
		userList = append(userList, user.ID)
		user = User{}
	}
	return userExists, userList
}

func deleteUser(c *gin.Context) {
	// The futur code…
}

func postProject(c *gin.Context) {

	projectData := Project{}

	body, _ := ioutil.ReadAll(c.Request.Body)

	err := json.Unmarshal(body, &projectData)
	if err != nil {
		panic(err)
	}

	project := Project{
		ProjectName: projectData.ProjectName,
		Author:      projectData.Author,
	}

	if project.Author == "" {
		c.JSON(400, StatusMessage{Error: "invalid user_name"})
	} else {
		DB.Create(&project)
	}

}

func getComponents(c *gin.Context) {

}
func getComponent(c *gin.Context) {

}
func postComponent(c *gin.Context) {

	componentData := Component{}

	body, _ := ioutil.ReadAll(c.Request.Body)

	err := json.Unmarshal(body, &componentData)
	if err != nil {
		panic(err)
	}

	projects := []Project{}
	DB.Find(&projects)

	projectExists, projectID := contains(componentData.ProjectName, projects)
	component := Component{
		ComponentName: componentData.ComponentName,
		ProjectName:   componentData.ProjectName,
		ProjectID:     projectID,
	}

	if projectExists {
		DB.Create(&component)
	} else {
		fmt.Println("nope")
	}

}

func updateComponents(c *gin.Context) {

}

func deleteComponent(c *gin.Context) {

}

func contains(name string, obj []Project) (bool, int) {
	for _, item := range obj {
		if item.ProjectName == name {
			return true, item.ID
		}
	}
	return false, 0
}
