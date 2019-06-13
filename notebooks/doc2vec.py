#%% Change working directory from the workspace root to the ipynb file location. Turn this addition off with the DataScience.changeDirOnImportExport setting
# ms-python.python added
import os
try:
	os.chdir(os.path.join(os.getcwd(), 'notebooks'))
	print(os.getcwd())
except:
	pass
#%% [markdown]
# # Building, training, and deploying Doc2Vec Model
#%% [markdown]
# ## Data preparation
# 
# You should download on GCS.
# 
# ```
# gs://your-own-name/train/
# ```

#%%
get_ipython().system('pip freeze')


#%%
deps = """
gensim
mecab-python3
neologdn
distro
pandas
joblib
numpy
seldon-core
google-cloud-storage
google-compute-engine
google-resumable-media[requests]
fairing==0.5.3
"""
with open("requirements.txt", 'w') as f:
    f.write(deps)
get_ipython().system('pip install -r requirements.txt')

#%% [markdown]
# ## Building a model and training it locally

#%%
import argparse
import logging
import joblib
import sys
import os
import re
import pandas as pd
import itertools

import MeCab
import neologdn
from gensim import models
from gensim.models.doc2vec import Doc2Vec
from gensim.models.doc2vec import TaggedDocument


#%%
logging.basicConfig(format='%(message)s')
logging.getLogger().setLevel(logging.INFO)

#%% [markdown]
# ## Creating a model class with train and predict methods

#%%
class Doc2VecServe(object):
    
    def __init__(self):
        self.mecab = MeCab.Tagger('-d/usr/lib/x86_64-linux-gnu/mecab/dic/mecab-ipadic-neologd -Ochasen')  # To install mecab by apt
        self.delimiter = "#DEMI#"
        self.train_input = "train.csv"
        self.model_file = "doc2vec.model"
        self.trained_model = None

    def split_into_words(self, text):
        words = []
        for chunk in self.mecab.parse(text).splitlines()[:-1]:
            cs = chunk.split('\t')
            words.append({
                'word': cs[0],
                'kind': cs[3]
            })
        return words

    def normalize(self, word_dic):
        tmp = word_dic['word']
        if word_dic['kind'] != '感動詞':
            tmp = neologdn.normalize(tmp)
        return tmp

    def pre_separate(self, text, url_type):
        tmp = text
        # 桁区切りの除去と数字の置換
        tmp = re.sub(r'(\d)([,.])(\d+)', r'\1\3', tmp)
        tmp = re.sub(r'\d+', '0', tmp)
        # URLの置換
        if url_type == "photo":
            tmp = re.sub(r'https?://[\w/:%#\$&\?\(\)~\.=\+\-]+', '.PHOTOIMAGE.', tmp)
        else:
            tmp = re.sub(r'https?://[\w/:%#\$&\?\(\)~\.=\+\-]+', '.QUOTATION.', tmp)
        # ハッシュタグの除去
        tmp = re.sub(r'#(\w+)', '', tmp)
        # リプ削除
        tmp = re.sub(r'@([A-Za-z0-9_]+)[ :]', '', tmp)
        return tmp


    def min_repeat(self, s, min_num):
        word_num = len(s)
        if word_num <= min_num:
            return s

        if (s[0] * word_num) != s:
            return s

        return s[0] * min_num

    def post_separate(self, dics):
        new_dics = []
        n = len(dics)
        pre = {'word': None, 'org': None, 'kind': []}
        for i in range(n):
            is_skip = False
            d = dics[i]
            word = d['word']
            try:
                if (len(new_dics) > 0) and (("接尾" in d['kind']) or ("接続助詞" in d['kind'])):
                    word = "{0}{1}".format(pre['word'], word)
                    new_dics[-1]['word'] = word
                    is_skip = True

                elif "名詞接続" in pre['kind'] and "名詞" in d['kind']:
                    word = "{0}{1}".format(pre['word'], word)
                    new_dics[-1]['word'] = word
                    is_skip = True

                elif ("句点" in d['kind']) or ("記号-空白" in d['kind']) or ("記号-読点" in d['kind']) or ('記号-括弧開' in d['kind']) or ('記号-括弧閉' in d['kind']):
                    is_skip = True

                elif "動詞-非自立" in d['kind']:
                    is_skip = True

                elif ("助動詞" in d['kind']) and (len(word) == 1):
                    is_skip = True

                elif (len(d['word']) == 1) and (pre['org'] == word):
                    word = "{0}{1}".format(pre['word'], word)
                    new_dics[-1]['word'] = word
                    is_skip = True

                elif ("助詞" in d['kind']) and (word in ["て", "に", "を", "は", "と", "も", "や", "の", "つつ", "ので"]):
                    is_skip = True

                elif ("記号" in d['kind']) and (len(word) == 1) and (word in ["・", "/", "＋", "(", ")", "#", "."]):
                    is_skip = True

                if "名詞-数" in d['kind']:
                    is_skip = True

            except:
                print("exception: {0}, word: {1}, new_dics: {2}".format(" ".join([x['word'] for x in dics]), word, len(new_dics)))
                raise

            word = self.min_repeat(word, 3)

            pre = {
                'word': word,
                'org' : d['word'],
                'kind': d['kind']
            }
            if not is_skip or (len(new_dics) == 0 and d['kind'] !='記号-括弧開'):
                d['word'] = word
                new_dics.append(d)
        return new_dics

    def extract_word(self, dics):
        return [x['word'] for x in dics]

    def concat_text(self, xs):
        res = itertools.chain(*xs)
        return list(res)

    def preprocess(self, text, url_type=""):
        wk = self.pre_separate(text, url_type)
        dics = self.split_into_words(wk)
        dics = self.post_separate(dics)
        return self.extract_word(dics)

    def train(self):
        df = pd.read_csv(self.train_input)
        df['doc'] = df.apply(lambda x: self.preprocess(x['tweet'], x['url_type']), axis=1)
        df = df[df['sex'].isin([0, 1])]

        ndf = df.reset_index()
        ndf['tagged'] = ndf.apply(lambda x: TaggedDocument(words=x['doc'], tags=[x['screen_name'], x['index']]), axis=1)

        sentences = list(ndf['tagged'])

        model = models.Doc2Vec(sentences, dm=0, vector_size=600, window=5, min_count=2, epochs=50, workers=4)
        model.save(self.model_file)

    def predict(self, X, feature_names=None):
        """Predict using the model for given string."""
        if not self.trained_model:
            self.trained_model = Doc2Vec.load(self.model_file)

        items = [X, ""]
        if self.delimiter in X:
            items = X.split('#DEMI#')
        vector = self.trained_model.infer_vector(self.preprocess(items[0], items[1]))
        return vector


#%% [markdown]
# ## Training Locally

#%%
get_ipython().run_cell_magic('time', '', 'Doc2VecServe().train()')


#%%
import fairing
GCP_PROJECT = fairing.cloud.gcp.guess_project_name()
GCS_BUCKET_ID = "your-own-name"
GCS_BUCKET = "gs://{}/model".format(GCS_BUCKET_ID)
get_ipython().system('gsutil ls {GCS_BUCKET}')


#%%
py_version = ".".join([str(x) for x in sys.version_info[0:3]])
print(py_version)

#%% [markdown]
# # Training and Deploying in Fairing
# 
# ### Setting up base container and builder for fairing
# 
# Setting up google container repositories (GCR) for storing output containers. You can use any docker container registry istead of GCR.

#%%
DOCKER_REGISTRY = 'gcr.io/{}/fairing/doc2vec'.format(GCP_PROJECT)

base_image = "gcr.io/{}/python3-mecab-pytorch:0.0.3".format(GCP_PROJECT)  # created in ahead.
fairing.config.set_builder('docker', registry=DOCKER_REGISTRY, base_image=base_image)


#%%
DEPLOYMENT_NAME='Your own name'
KUBEFLOW_ZONE='Your own zone'

#%%
get_ipython().system('gcloud config set project $GCP_PROJECT')


#%%
get_ipython().system("gcloud container clusters get-credentials $DEPLOYMENT_NAME --zone $KUBEFLOW_ZONE --project $GCP_PROJECT")


#%%
get_ipython().system('kubectl config set-context $(kubectl config current-context) --namespace=kubeflow')


#%%
get_ipython().system('gcloud auth configure-docker --quiet')

#%% [markdown]
# ## Training in KF

#%%
fairing.config.set_deployer('job')
fairing.config.set_preprocessor("function", function_obj=Doc2VecServe,
                                input_files=["requirements.txt", "tweets/tweets.csv"])
fairing.config.run()

#%% [markdown]
# ## Deploying model and creating an endpoint in KF

#%%
fairing.config.set_preprocessor("function", function_obj=Doc2VecServe,
                                input_files=["requirements.txt", "doc2vec.model", "doc2vec.model.trainables.syn1neg.npy", "doc2vec.model.wv.vectors.npy", "doc2vec.model.docvecs.vectors_docs.npy"])
fairing.config.set_deployer('serving', serving_class="Doc2VecServe")
fairing.config.run()


#%%
# Copy the prediction endpoint from prev step
get_ipython().system('curl -g http://xx.xx.xx.xx:5000/predict -H "Content-Type: application/x-www-form-urlencoded" --data-urlencode \'json={"strData":"めちゃ楽しい一日でした＼(^o^)／"}\' -vv')


#%%
Doc2VecServe().predict("私は女性でしょうか？男性でしょうか？#DEMI#")


