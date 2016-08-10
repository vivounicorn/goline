
ftrl-proximal是一个online learning工具库，其实现基于以下两篇论文：
* 《Ad Click Prediction: a View from the Trenches》 from Brendan McMahan...(google)
* 《Large Scale Distributed Deep Networks》 from Jeffrey Dean...(google)

Features
----------

* 支持Libsvm格式
* 支持较大规模训练数据
* 支持原生ftrl-proximal算法
* 支持多核多线程的ftrl-proximal算法
* 支持基于parameter server的快速ftrl-proximal算法

Usage
----------
* 支持基于parameter server的快速ftrl-proximal算法(推荐)

初始化参数说明:
	epoch 				迭代轮数
	num_threads 		线程数(设置为0默认获取CPU核心数)
	cache_feature_num 	是否缓存样本统计信息
	burn_in 			第一次迭代时的模型权重预热
	push_step 			训练多少步后向参数服务器推送更新梯度值
	fetch_step int		训练多少步后从参数服务器获取更新梯度值

训练参数说明：
	alpha 				权重更新步长的参数，由样本数和特征数决定(用于确定更新权重步长)
	beta 				权重更新步长的参数，一般设置为1即可(用于确定更新权重步长)
	l1					L1正则化系数(保持模型稀疏性及提高泛化性)
	l2					L2正则化系数(保持模型参数稳定性及提高泛化性)
	dropout				设置放弃更新某个特征权重的概率，取值为0~1之间(提高泛化性)
	sample				设置抽样比例，取值为-1~1之间，大于0时为样本整体抽样比例，小于0时为负样本抽样比例
	model_file			训练结束后模型文件存储路径
	train_file			训练文件存储路径
	test_file			测试文件存储路径

示例:
	var fft trainer.FastFtrlTrainer
	fft.Initialize(5, 8, false, 1, 10, 10)
	fft.Train(0.1,1,10,10,0.1,"..\\demo\\t.model",
	"..\\demo\\train.dat",
	"..\\demo\\test.dat")
	
	predictor.Run(3,[]string{"..\\demo\\using.dat",
	"..\\demo\\t.model",
	"..\\demo\\res.dat",
	"0.06"})
	
* 支持原生ftrl-proximal算法
	var ft trainer.FtrlTrainer
	ft.Initialize(5, false)
	ft.Train(0.1,1,10,10,0.1,"..\\demo\\t.model",
	"..\\demo\\train.dat",
	"..\\demo\\test.dat")
	
	predictor.Run(3,[]string{"..\\demo\\using.dat",
	"..\\demo\\t.model",
	"..\\demo\\res.dat",
	"0.06"})

* 支持多核多线程的ftrl-proximal算法
	var lft trainer.LockFreeFtrlTrainer
	lft.Initialize(5, 8, false)
	lft.Train(0.1,1,10,10,0.1,"..\\demo\\t.model",
	"..\\demo\\train.dat",
	"..\\demo\\test.dat")
	
	predictor.Run(3,[]string{"..\\demo\\using.dat",
	"..\\demo\\t.model",
	"..\\demo\\res.dat",
	"0.06"})

Future Features
----------

* 添加支持Single样本或batch样本的online learning的接口
* 多线程并发性能优化

Http Service
----------
* 训练离线模型——使用方法
http://127.0.0.1:8080/offline?biz=[model name]&src=[hdfs/local]&dst=[redis&local&json]
              &alpha=[0.1]&beta=[0.1]&l1=[10]&l2=[10]&dropout=[0.1]&epoch=[2]
		&push=[push step]&fetch=[fetch step]&threads=[threads number]
		&train=[train file name]&test=[test file name]&debug=[off]&thd=[threshold]
 src:训练、测试数据源为hdfs/local
 dst:模型输出到redis、local和json
 train:训练数据完整路径
 test:测试数据完整路径
例如：		http://192.168.225.130/ftrl/offline?biz=model2&src=hdfs&dst=json&alpha=0.1&beta=0.1&l1=10&l2=100&dropout=0.1&sample=0.1&epoch=1&push=20&fetch=20&threads=8&train=/dmp/tmp/clue_level_predict/feature_model/mds/tmp_mds_dm_clue_user_feature_data_for_LPU/src=train&test=/dmp/tmp/clue_level_predict/feature_model/mds/tmp_mds_dm_clue_user_feature_data_for_LPU/src=test&thd=0.06

* 在线学习——使用方法
http://127.0.0.1:8080/online?biz=[model name]&src=[redis&stream]&dst=[redis&local&json]
              &epoch=[2]&threads=[threads number]&train=[redis key/instance strings]
		&debug=[off]&thd=[threshold]
 src:训练数据为redis还是即时stream (初始化模型如果redis不存在则读取local,模型key为biz值)
 dst:模型存储到redis、local和json
 train:训练数据来源于redis或stream
例如：http://192.168.225.130/ftrl/online?biz=model1&src=redis&dst=json&alpha=0.1&beta=0.1&l1=10&l2=100&dropout=0.1&sample=0.1&epoch=100&push=5&fetch=5&threads=4&train=0%2040:1%2091:1%20145:1%20195:1%20244:1%20294:1%20340:1%20374:1%20404:1%20460:1%20500:1%20556:1%20608:1%20611:1%20661:1%20711:1%20799:1,%200%2047:1%2097:1%20144:1%20198:1%20246:1%20299:1%20347:1%20377:1%20408:1%20457:1%20510:1%20537:1%20610:1%20659:1%20703:1%20757:1%20788:1,%201%201:1%2051:1%20101:1%20151:1%20201:1%20251:1%20301:1%20351:1%20381:1%20411:1%20461:1%20556:1%20561:1%20647:1%20699:1%20711:1%20808:1,%201%201:1%2051:1%20101:1%20151:1%20201:1%20251:1%20301:1%20351:1%20381:1%20411:1%20461:1%20556:1%20561:1%20647:1%20699:1%20711:1%20808:1,%200%2048:1%2098:1%20145:1%20194:1%20242:1%20289:1%20347:1%20377:1%20405:1%20411:1%20461:1%20550:1%20561:1%20611:1%20701:1%20711:1%20805:1,%200%2037:1%2088:1%20137:1%20190:1%20239:1%20292:1%20341:1%20376:1%20407:1%20439:1%20509:1%20529:1%20607:1%20643:1%20685:1%20753:1%20793:1&thd=0.06

* 在线预估——使用方法
http://127.0.0.1:8080/predict?biz=[model name]&src=[hdfs/redis/stream]&dst=[local&json]
		&pred=[hdfs file/redis key/instance strings]&debug=[off]&thd=[threshold]
 src:待预测数据源为hdfs/local/stream
 dst:待预测数据输出到local和json
 pred:待预测数据完整路径
例如：
1、数据源为：hdfs
http://192.168.225.130/ftrl/predict?biz=model2&src=hdfs&dst=json&predict=/dmp/tmp/clue_level_predict/feature_model/mds/tmp_mds_dm_clue_user_feature_data_for_LPU/src=test&thd=0.06
2、数据源为：stream
http://192.168.225.130/ftrl/predict?biz=model2&src=stream&dst=json&predict=0%2040:1%2091:1%20145:1%20195:1%20244:1%20294:1%20340:1%20374:1%20404:1%20460:1%20500:1%20556:1%20608:1%20611:1%20661:1%20711:1%20799:1&thd=0.06

License
----------

© Contributors, 2015. Licensed under an Apache-2 license.
