package services

import ( 
	"strconv"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"github.com/vonmutinda/crafted/api/channels"
	"github.com/vonmutinda/crafted/api/database"
	"github.com/vonmutinda/crafted/api/messages"
	"github.com/vonmutinda/crafted/api/models"
)

type ArticleCRUD struct {
	L *logrus.Logger
}
 

func (repo *ArticleCRUD) GetAllArticles() ([]models.Article, error){
	var err error

	// articles slice
	articles := []models.Article{}

	// channels
	done := make(chan bool)

	// go routine to fetch articles
	go func(c chan<- bool){   
		if err = database.GetDB().Preload("Author").Find(&articles).Error ; err != nil {
			repo.L.Error(err)
		}
		c<- true 
	}(done) 

	if channels.OK(done){ 
		return articles,nil 
	}
	return nil, err
}

// save a new article 
func (repo *ArticleCRUD) SaveArticle(article models.Article) (models.Article, error){
	var err error 
	
	done := make(chan bool)
	go func(c chan<- bool){
		err = database.GetDB().Debug().Model(&models.Article{}).Create(&article).Error 
		if err != nil {  
			c<- false
			return
		} 
		
		err = database.GetDB().Where("id = ?", article.AuthorID).Take(&article.Author).Error
		if err != nil {
			repo.L.Error(err)
			c<- false
		}
		c<- true 
	}(done)

	if channels.OK(done){ 
		return article,nil 
	}
	return models.Article{}, err
}

// delete all articles 
func (repo *ArticleCRUD) DeleteAllArticles() (int64, error){
	// var err error 
	var rep *gorm.DB
	done := make(chan int, 1)

	go func(c chan<- int){  
		rep = database.GetDB().Raw(`
			UPDATE articles
			SET deleted_at=? 
		`, time.Now())
		c<- 1
	}(done)
	
	<-done

	return rep.RowsAffected, rep.Error
}

// find article by id 
func (repo *ArticleCRUD) FetchArticleByID(id uint64) (models.Article, error){ 

	var err error 
	article := models.Article{}  
	done := make(chan bool)

	go func(c chan<- bool){
		if err = database.GetDB().Preload("Author").Where("ID = ?", id).Take(&article).Error; err != nil {
			repo.L.Error(err)
			c<- false
			return
		} 
		c<- true
	}(done)

	if channels.OK(done){ 
		return article, nil
	}

	if gorm.IsRecordNotFoundError(err){ 
		return models.Article{}, err
	}

	return models.Article{}, err	
}

// delete article by ID
func (repo *ArticleCRUD) DeleteByID(id uint64) (int64, error){ 
	var rep *gorm.DB

	done := make(chan int, 1) 
	go func(c chan<- int){ 
		rep = database.GetDB().Where("id = ?", id).Delete(&models.Article{}) 
		c<- 1
	}(done)

	<-done 
	return rep.RowsAffected, rep.Error 
}


// update article - We'll use waitgroups
func(repo *ArticleCRUD) UpdateArticle(updated models.Article, aid int64)(int64, error){

	var gor *gorm.DB
	var wg sync.WaitGroup 

	wg.Add(1)
	go func(done *sync.WaitGroup){

		defer done.Done()  

		// for testing purpose let's delegate updating time to rabbitmq
		gor = database.GetDB().Exec(`
				UPDATE articles
				SET title=?,
					body=?
				WHERE id=?
			`,updated.Title,
			updated.Body,
			aid,
		) 
 
	}(&wg)

	wg.Wait() 
	
	// send to queue 
	s := strconv.FormatInt(aid, 10)
	messages.SendMessage("updated_at", s)

	return gor.RowsAffected, gor.Error
} 