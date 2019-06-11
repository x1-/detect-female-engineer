# Detect Engineer Client


Call API of the Doc2Vec model based on seldon and kubeflow.  
And, Call API of the LSTM model based on seldon and kubeflow.  

# Prereuists

You should use go command installing dependencies.

```sh
$ go get -u github.com/gin-gonic/gin

$ go get -u github.com/go-sql-driver/mysql
$ go get -u -t github.com/volatiletech/sqlboiler
$ go get github.com/volatiletech/sqlboiler/drivers/sqlboiler-mysql
$ go get github.com/volatiletech/null

$ go get github.com/ChimeraCoder/anaconda
$ go get github.com/sirupsen/logrus
```

# Dependency

- golang >= 1.10
- [gin](https://github.com/gin-gonic/gin)
- [sqlboiler](https://github.com/volatiletech/sqlboiler)
- [anaconda](https://github.com/ChimeraCoder/anaconda)
- [logrus](https://github.com/sirupsen/logrus)

# Usage

```sh
DB_NAME="dummy"
DB_USER="dummy"
DB_PASSWD="dummy"
CONSUMER_KEY="dummy"
CONSUMER_SECRET="dummy"
ACCESS_TOKEN="dummy"
ACCESS_TOKEN_SECRET="dummy"
DOC2VEC_ENDPOINT="http://127.0.0.1:5000/predict"
FEMALE_ENDPOINT="http://127.0.0.1:5000/predict"
ENGINEER_ENDPOINT="http://127.0.0.1:5000/predict"

go run server.go \
  -db_name $DB_NAME \
  -db_user $DB_USER \
  -db_passwd $DB_PASSWD \
  -consumer_key $CONSUMER_KEY \
  -consumer_secret $CONSUMER_SECRET \
  -access_token $ACCESS_TOKEN \
  -access_token_secret $ACCESS_TOKEN_SECRET \
  -doc2vec_endpoint $DOC2VEC_ENDPOINT \
  -female_endpoint $FEMALE_ENDPOINT \
  -engineer_endpoint $ENGINEER_ENDPOINT
```
