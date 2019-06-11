CREATE TABLE `access` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `account` varchar(255) NOT NULL DEFAULT '',
  `tweet` varchar(255) NOT NULL DEFAULT '',
  `predicted_sex` int(11) NOT NULL,
  `probability_sex` double NOT NULL,
  `predicted_engineer` int(11) NOT NULL,
  `probability_engineer` double NOT NULL,
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
