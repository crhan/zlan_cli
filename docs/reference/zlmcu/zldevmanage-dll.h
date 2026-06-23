/* 该文件定义了调用ZlDevManageDll的宏定义和函数 */
#ifndef __INCLUDE_ZL_TYPES_H__


#endif

/* 宏定义
*/
#define P2P_EN							0
#define ZLDM_VER						"V1.46"//"V1.45_P2P"			// 库版本号，如果含有P2P字眼则是会成为含有p2p功能的库，比如"V1.41_P2P"
#define ID_LEN							6				// hex型的设备id的长度
#define ID_STRING_LEN					(ID_LEN*2)		// 字符串型的设备id的长度
#define ZLDM_HANDLER_ARRAY_MIN_SIZE		256				// 定义的设备总数，可以被256更大

/* 以下宏定义都是获得、设置参数时的参数类型定义，详细参考《开发库文档》表1
*/

#define PARAM_SET_TO_DEFAULT			0			

#define PARAM_DEV_EXIST_IN_SUBNET		1					
#define PARAM_DEV_LOCAL_IP				2			
#define PARAM_DEV_LOCAL_PORT			3			
#define PARAM_DEV_OUTER_IP              		4			
#define PARAM_ALL_SUBNET_ID             		5			
#define PARAM_SIMULATE_PORT             		6			
#define PARAM_P2P_STATUS_CODE			7			
#define PARAM_P2P_STATUS_CHS			8			
#define PARAM_P2P_STATUS_EN			9		

#define PARAM_WORK_MODE				256			
#define PARAM_IP_MODE					257
#define PARAM_FLOW_CONTROL			258
#define PARAM_DEST_MODE				259	
#define PARAM_APP_PROTOCOL				260
#define PARAM_STOP_BIT					261
#define PARAM_PARITY					262
#define PARAM_DATA_BITS					263
#define PARAM_BAUNDRATE					264
#define PARAM_DNS_SERVER_IP				265
#define PARAM_UDP_GROUP_IP				266
//#define PARAM_LOCAL_IP					267	
#define PARAM_NET_MASK					268
#define PARAM_GATEWAY					269
#define PARAM_RECONNECT_TIME			270
#define PARAM_KEEP_ALIVE_TIME			271
#define PARAM_WEB_PORT					272
#define PARAM_DEV_VER					273
//#define PARAM_LOCAL_PORT				274
#define PARAM_DEST_PORT					275
#define PARAM_DEV_NAME					276
#define PARAM_DEST_IP					277
#define PARAM_GAP_TIME					278
#define PARAM_PACKING_LEN				279
#define PARAM_LINK_STATUS				280
#define PARAM_RESOURCE_IP				281
#define PARAM_RESOURCE_PORT			282

// RS485多主机支持
#define PARAM_485_TIME_OUT				283
#define PARAM_485_GAP					284
#define PARAM_RECOMMAND_485_TIME_OUT	285
#define PARAM_RECOMMAND_485_GAP			286
#define PARAM_RECOMMAND_GAP_TIME		287
#define PARAM_KEY_FOR_ALLOW_MODIFY	288

// 设备支持的功能
#define PARAM_FUNC_MODBUS				200
#define PARAM_FUNC_P2P					201

// 设备功能启用
#define PARAM_ENABLE_P2P				220
#define PARAM_ENABLE_SEND_MAC			221
#define PARAM_ENABLE_TCP_NEED_KEY		222
#define PARAM_ENABLE_UDP_MODIFY_NEED_KEY	223

// wifi参数
#define PARAM_WIFI_SSID					350
#define PARAM_WIFI_CHANNEL				351
#define PARAM_WIFI_KEY					352
#define PARAM_WIFI_KEY_TYPE				353
#define PARAM_WIFI_STA_AP				354
#define PARAM_WIFI_DHCP_SERVER			355
#define PARAM_WIFI_ETH_BRIDGE			356


// 密码验证的宏定义
#define KEY_AUTH_BUF_LEN				37
#define KEY_AUTH_CRYPTION_LEN			16	// 随机码或者加密结果的长度
#define KEY_AUTH_POS_RANDOM			21	// 随机码所在位置
#define KEY_AUTH_POS_CMD				4	// 命令码所在位置
#define KEY_AUTH_POS_ID					7	// ID所在位置

#ifndef P2P_GET_STATE_UNKOWN
#define P2P_GET_STATE_UNKOWN		0		// 未知
#define P2P_GET_STATE_UNCONECTED	1		// 和服务器未连接
#define P2P_GET_STATE_PORT_ERR		2		// 输入的端口不存在
#define P2P_GET_STATE_SUBNET		3		// 内网
#define P2P_GET_STATE_INTERNET		4		// 外网
#define P2P_GET_STATE_PROXY			5		// 代理
#define P2P_GET_STATE_CONECTTING	6		// 连接中或者等待连接
#define P2P_GET_STATE_WAIT_CON		7		// 等待p2p连接到来
#define P2P_GET_STATE_SERVER		8		// 是服务器模式，做作为pc通过这个变量可以知道是否调用了p2popen函数
#define P2P_GET_STATE_SERVER_AUTH_ERR	9	// pc或者服务器未验证服务器的合法性
#define P2P_GET_STATE_DEV_OFF_LINE	10		// pc连接的dev处于离线状态
#define P2P_GET_STATE_DEV_AUTH_ERR	11		// pc连接的dev未被服务器验证通过
#define P2P_GET_STATE_PC_AUTH_ERR	12		// pc未被服务器验证通过
#define P2P_GET_STATE_DEV_BELONG_ERR	13	// pc希望连接的dev不属于这个用户所有
#define P2P_GET_STATE_SERVER_NO_ACK	14	// 发送请求给服务器，但是没有应答
#define P2P_GET_STATE_DEV_NO_ACK		15	// 发送请求后服务器有应答，设备没有应答
#define P2P_GET_STATE_WAIT_PROXY		16	// 尝试通过代理方式连接
#endif

/* 函数指针定义 */
typedef int	(__stdcall * tZLDM_Init)(int ServerPort);
typedef void	(__stdcall * tZLDM_Exit)();
typedef char *	(__stdcall * tZLDM_GetVer)();

typedef int	(__stdcall * tZLDM_StartSearchDev)();
typedef char * (__stdcall * tZLDM_GetDevID)(int Dev_Index);

typedef int (__stdcall * tZLDM_AddManualDev)(const char ip_dns1[], const char ip_dns2[], int port);
typedef void	(__stdcall * tZLDM_ClearManualDev)();

typedef  char * (__stdcall *tZLDM_GetDevParamString)(const char *id, int ParamType);
typedef  int (__stdcall *tZLDM_GetDevParamInt)(const char *id, int ParamType);
typedef  int (__stdcall *tZLDM_SetDevParamString)(const char *id, const char *NewParam, int ParamType);
typedef  int (__stdcall *tZLDM_SetDevParamInt)(const char *id, int NewParam, int ParamType);
typedef  int (__stdcall *tZLDM_SetDevParamExcute)(const char *id);
typedef  int (__stdcall *tZLDM_ModifyKey)(const char *id2, const char *OldKey, const char *NewKey);
typedef  int (__stdcall *tZLDM_GetKeyCmdType)(unsigned char *recv_buf);
typedef  unsigned char * (__stdcall *tZLDM_Random)();
typedef  unsigned char * (__stdcall *tZLDM_Crypt)(unsigned char *rand, const char * key);


/* p2p functions */
typedef  bool (__stdcall *tZLDM_P2pOpen)(const char* svraddr,int svrport, const char* username ,const char* userkey, int retry_time);
typedef  bool (__stdcall *tZLDM_P2pClose)();
typedef  int (__stdcall *tZLDM_P2pAddPort)(int simulate_port, const char* dev_id, const char *connect_to_ip, unsigned short connect_to_port);
typedef  bool (__stdcall *tZLDM_P2pDelPort)(int simulate_port);
typedef  void (__stdcall *tZLDM_P2pRestartAll)();
typedef  bool (__stdcall *tZLDM_P2pRetartPort)(int simulate_port);
typedef  char * (__stdcall *tZLDM_P2pGetID)(int ithP2pDev);



