// DlgParam.cpp : implementation file
//

#include "stdafx.h"
#include "ZLUseDevManage.h"
#include "DlgParam.h"

#ifdef _DEBUG
#define new DEBUG_NEW
#undef THIS_FILE
static char THIS_FILE[] = __FILE__;
#endif

/////////////////////////////////////////////////////////////////////////////
// CDlgParam dialog
extern CString g_pCurSelID;

extern tZLDM_GetDevParamString		m_pZLDM_GetDevParamString;
extern tZLDM_GetDevParamInt		m_pZLDM_GetDevParamInt;
extern tZLDM_SetDevParamString		m_pZLDM_SetDevParamString;
extern tZLDM_SetDevParamInt		m_pZLDM_SetDevParamInt;
extern tZLDM_SetDevParamExcute		m_pZLDM_SetDevParamExcute;
extern tZLDM_ModifyKey	m_pZLDM_ModifyKey;

CDlgParam::CDlgParam(CWnd* pParent /*=NULL*/)
	: CDialog(CDlgParam::IDD, pParent)
{
	//{{AFX_DATA_INIT(CDlgParam)
	m_DevName = _T("");
	m_DevDestPort = 0;
	m_dev_gap_time = 0;
	m_DevLocalPort = 0;
	m_dev_packing_len = 0;
	m_DestIP = _T("");
	m_ReconnectTime = 0;
	m_KeepAliveTime = 0;
	m_WebPort = 0;
	m_dev_ver = _T("");
	m_DevGateway = _T("");
	m_DevLocalIP = _T("");
	m_DnsServerIP = _T("");
	m_DevUDPGroupIP = _T("");
	m_DevNetmask = _T("");
	m_485HalfGap = 0;
	m_EnableP2p = FALSE;
	m_EnableSendMac = FALSE;
	m_MultiHostTimeOut = 0;
	m_FuncSelModbusTCP = FALSE;
	m_FuncSel2P2p = FALSE;
	m_EnableTCPNeedKey = FALSE;
	m_UDPModifyNeedKey = FALSE;
	m_InputKey = _T("");
	m_WifiSSID = _T("");
	m_WifiKey = _T("");
	//}}AFX_DATA_INIT
}


void CDlgParam::DoDataExchange(CDataExchange* pDX)
{
	CDialog::DoDataExchange(pDX);
	//{{AFX_DATA_MAP(CDlgParam)
	DDX_Control(pDX, IDC_WIFI_CHANNEL, m_WifiChannel);
	DDX_Control(pDX, IDC_WIFI_DHCP_SERVER, m_WifiDHCPServer);
	DDX_Control(pDX, IDC_WIFI_ETH_WIFI_BRIDGE, m_WifiEthWifiBridge);
	DDX_Control(pDX, IDC_WIFI_KEY_TYPE, m_WifiKeyType);
	DDX_Control(pDX, IDC_WIFI_STA_AP, m_WifiSTAAP);
	DDX_Control(pDX, IDC_DEV_APP_PROTOCOL, m_DevAppProtocol);
	DDX_Control(pDX, IDC_DEV_DESTMODE, m_DevDestMode);
	DDX_Control(pDX, IDC_DEV_FLOW_CONTROL, m_DevFlowControl);
	DDX_Control(pDX, IDC_DEV_IPMODE, m_DevIPMode);
	DDX_Control(pDX, IDC_DEV_WORKMODE, m_DevWorkMode);
	DDX_Control(pDX, IDC_DEV_STOPBIT, m_DevStopBit);
	DDX_Control(pDX, IDC_DEV_PARITY, m_DevParity);
	DDX_Control(pDX, IDC_DEV_BITNUM, m_DevBitNum);
	DDX_Control(pDX, IDC_DEV_BAUNDRATE, m_DevBaundrate);
	DDX_Text(pDX, IDC_DEV_NAME, m_DevName);
	DDV_MaxChars(pDX, m_DevName, 9);
	DDX_Text(pDX, IDC_DEV_DESTPORT, m_DevDestPort);
	DDV_MinMaxUInt(pDX, m_DevDestPort, 0, 65535);
	DDX_Text(pDX, IDC_DEV_GAP_TIME, m_dev_gap_time);
	DDV_MinMaxUInt(pDX, m_dev_gap_time, 0, 255);
	DDX_Text(pDX, IDC_DEV_LOCAL_PORT, m_DevLocalPort);
	DDV_MinMaxUInt(pDX, m_DevLocalPort, 0, 65535);
	DDX_Text(pDX, IDC_DEV_PACKING_LEN, m_dev_packing_len);
	DDV_MinMaxUInt(pDX, m_dev_packing_len, 0, 1400);
	DDX_Text(pDX, IDC_DEST_IP, m_DestIP);
	DDV_MaxChars(pDX, m_DestIP, 29);
	DDX_Text(pDX, IDC_RECONNECT_TIME, m_ReconnectTime);
	DDV_MinMaxUInt(pDX, m_ReconnectTime, 0, 255);
	DDX_Text(pDX, IDC_KEEP_ALIVE_TIME, m_KeepAliveTime);
	DDV_MinMaxUInt(pDX, m_KeepAliveTime, 0, 255);
	DDX_Text(pDX, IDC_WEB_PORT, m_WebPort);
	DDV_MinMaxUInt(pDX, m_WebPort, 0, 65535);
	DDX_Text(pDX, IDC_DEV_VER, m_dev_ver);
	DDV_MaxChars(pDX, m_dev_ver, 8);
	DDX_Text(pDX, IDC_DEV_GATEWAY, m_DevGateway);
	DDX_Text(pDX, IDC_DEV_LOCAL_IP, m_DevLocalIP);
	DDX_Text(pDX, IDC_DNS_SERVER_IP, m_DnsServerIP);
	DDX_Text(pDX, IDC_DEV_UDP_GROUP_IP, m_DevUDPGroupIP);
	DDX_Text(pDX, IDC_DEV_NETMASK, m_DevNetmask);
	DDX_Text(pDX, IDC_485_HALF_GAP, m_485HalfGap);
	DDX_Check(pDX, IDC_ENABLE_P2P, m_EnableP2p);
	DDX_Check(pDX, IDC_ENABLE_SEND_MAC, m_EnableSendMac);
	DDX_Text(pDX, IDC_MULTI_HOST_TIME_OUT, m_MultiHostTimeOut);
	DDX_Check(pDX, IDC_FUNC_SEL_MODBUS_TCP, m_FuncSelModbusTCP);
	DDX_Check(pDX, IDC_FUNC_SEL2_P2P, m_FuncSel2P2p);
	DDX_Check(pDX, IDC_ENABLE_TCP_NEED_KEY, m_EnableTCPNeedKey);
	DDX_Check(pDX, IDC_UDP_MODIFY_NEED_KEY, m_UDPModifyNeedKey);
	DDX_Text(pDX, IDC_INPUT_KEY, m_InputKey);
	DDV_MaxChars(pDX, m_InputKey, 10);
	DDX_Text(pDX, IDC_WIFI_SSID, m_WifiSSID);
	DDX_Text(pDX, IDC_WIFI_KEY, m_WifiKey);
	//}}AFX_DATA_MAP
}


BEGIN_MESSAGE_MAP(CDlgParam, CDialog)
	//{{AFX_MSG_MAP(CDlgParam)
	ON_BN_CLICKED(ID_DEF_PARAM, OnDefParam)
	ON_CBN_SELCHANGE(IDC_DEV_APP_PROTOCOL, OnSelchangeDevAppProtocol)
	//}}AFX_MSG_MAP
END_MESSAGE_MAP()

void CDlgParam::OnOK() 
{
	CString str;
	UINT i;

	UpdateData(TRUE);

	// 修改需要密码的时候需要输入正确密码
	if((*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_ENABLE_UDP_MODIFY_NEED_KEY) == TRUE)
	{
		AfxMessageBox("Modify need key, please input correct key!");
		(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_InputKey, PARAM_KEY_FOR_ALLOW_MODIFY);
	}

	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevWorkMode.GetCurSel(), PARAM_WORK_MODE);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevIPMode.GetCurSel(), PARAM_IP_MODE);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevFlowControl.GetCurSel(), PARAM_FLOW_CONTROL);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevDestMode.GetCurSel(), PARAM_DEST_MODE);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevBaundrate.GetCurSel(), PARAM_BAUNDRATE);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevParity.GetCurSel(), PARAM_PARITY);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevBitNum.GetCurSel(), PARAM_DATA_BITS);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevAppProtocol.GetCurSel(), PARAM_APP_PROTOCOL);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevStopBit.GetCurSel(), PARAM_STOP_BIT);
	
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_DnsServerIP, PARAM_DNS_SERVER_IP);
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_DevUDPGroupIP, PARAM_UDP_GROUP_IP);
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_DevLocalIP, PARAM_DEV_LOCAL_IP);
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_DevNetmask, PARAM_NET_MASK);
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_DevGateway, PARAM_GATEWAY);
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_DevName, PARAM_DEV_NAME);
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_DestIP, PARAM_DEST_IP);

	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_ReconnectTime, PARAM_RECONNECT_TIME);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_KeepAliveTime, PARAM_KEEP_ALIVE_TIME);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WebPort, PARAM_WEB_PORT);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevLocalPort, PARAM_DEV_LOCAL_PORT);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_DevDestPort, PARAM_DEST_PORT);

	// 设置的值应该大于推荐的值
	i = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_RECOMMAND_GAP_TIME);
	if(m_dev_gap_time > i)
		i = m_dev_gap_time;
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, i, PARAM_GAP_TIME);

	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_dev_packing_len, PARAM_PACKING_LEN);

	// 功能选择
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_EnableP2p, PARAM_ENABLE_P2P);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_EnableSendMac, PARAM_ENABLE_SEND_MAC);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_EnableTCPNeedKey, PARAM_ENABLE_TCP_NEED_KEY);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_UDPModifyNeedKey, PARAM_ENABLE_UDP_MODIFY_NEED_KEY);

	// 485多主机，设置的值小于等于推荐值则不使用

	i = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_RECOMMAND_485_TIME_OUT);
	if(m_MultiHostTimeOut > i)
		i = m_MultiHostTimeOut;
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, i, PARAM_485_TIME_OUT);

	i = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_RECOMMAND_485_GAP);
	if(m_485HalfGap > i)
		i = m_485HalfGap;
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, i, PARAM_485_GAP);

	// wifi参数 
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_WifiSSID, PARAM_WIFI_SSID);
	(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_WifiKey, PARAM_WIFI_KEY);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiChannel.GetCurSel()+1, PARAM_WIFI_CHANNEL);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiDHCPServer.GetCurSel(), PARAM_WIFI_DHCP_SERVER);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiEthWifiBridge.GetCurSel(), PARAM_WIFI_ETH_BRIDGE);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiKeyType.GetCurSel(), PARAM_WIFI_KEY_TYPE);
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiSTAAP.GetCurSel(), PARAM_WIFI_STA_AP);

	// 执行以上的写入的结果
	(*m_pZLDM_SetDevParamExcute)(g_pCurSelID);
	CDialog::OnOK();
}
/////////////////////////////////////////////////////////////////////////////
// CDlgParam message handlers

BOOL CDlgParam::OnInitDialog() 
{
	CString str;
	int value;

	CDialog::OnInitDialog();
	
	m_DevWorkMode.InsertString(-1,"TCP Server");
	m_DevWorkMode.InsertString(-1,"TCP Client");
	m_DevWorkMode.InsertString(-1,"UDP");
	m_DevWorkMode.InsertString(-1,"UDP Group");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_WORK_MODE);
	m_DevWorkMode.SetCurSel(value);
	
	m_DevIPMode.InsertString(-1,"static");
	m_DevIPMode.InsertString(-1,"DHCP");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_IP_MODE);
	m_DevIPMode.SetCurSel(value);

	m_DevFlowControl.InsertString(-1,"None");
	m_DevFlowControl.InsertString(-1,"CTS/RTS");
	m_DevFlowControl.InsertString(-1,"DSR/DTR");
	m_DevFlowControl.InsertString(-1,"XON/XOFF");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_FLOW_CONTROL);
	m_DevFlowControl.SetCurSel(value);

	m_DevDestMode.InsertString(-1,"Static");
	m_DevDestMode.InsertString(-1,"Dynamic");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_DEST_MODE);
	m_DevDestMode.SetCurSel(value);

	m_DevAppProtocol.InsertString(-1,"NONE");
	m_DevAppProtocol.InsertString(-1,"Modbus TCP<->RTU");
	m_DevAppProtocol.InsertString(-1,"REAL_COM");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_APP_PROTOCOL);
	m_DevAppProtocol.SetCurSel(value);

	m_DevStopBit.InsertString(-1,"1");
	m_DevStopBit.InsertString(-1,"2");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_STOP_BIT);
	m_DevStopBit.SetCurSel(value);
	
	m_DevParity.InsertString(-1,"None");
	m_DevParity.InsertString(-1,"Odd");
	m_DevParity.InsertString(-1,"Even");
	m_DevParity.InsertString(-1,"Mark");
	m_DevParity.InsertString(-1,"Space");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_PARITY);
	m_DevParity.SetCurSel(value);
	
	m_DevBitNum.InsertString(-1,"8");
	m_DevBitNum.InsertString(-1,"7");
	m_DevBitNum.InsertString(-1,"6");
	m_DevBitNum.InsertString(-1,"5");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_DATA_BITS);
	m_DevBitNum.SetCurSel(value);
	
	m_DevBaundrate.InsertString(-1,"1200");
	m_DevBaundrate.InsertString(-1,"2400");
	m_DevBaundrate.InsertString(-1,"4800");
	m_DevBaundrate.InsertString(-1,"7200");
	m_DevBaundrate.InsertString(-1,"9600");
	m_DevBaundrate.InsertString(-1,"14400");
	m_DevBaundrate.InsertString(-1,"19200");
	m_DevBaundrate.InsertString(-1,"28800");
	m_DevBaundrate.InsertString(-1,"38400");
	m_DevBaundrate.InsertString(-1,"57600");
	m_DevBaundrate.InsertString(-1,"76800");
	m_DevBaundrate.InsertString(-1,"115200");
	m_DevBaundrate.InsertString(-1,"230400");
	m_DevBaundrate.InsertString(-1,"460800");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_BAUNDRATE);
	m_DevBaundrate.SetCurSel(value);

	m_DnsServerIP = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_DNS_SERVER_IP);
	m_DevUDPGroupIP = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_UDP_GROUP_IP);
	m_DevLocalIP = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_DEV_LOCAL_IP);
	m_DevNetmask = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_NET_MASK);
	m_DevGateway = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_GATEWAY);

	m_ReconnectTime = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_RECONNECT_TIME);
	m_KeepAliveTime = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_KEEP_ALIVE_TIME);
	m_WebPort       = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_WEB_PORT);

	m_dev_ver = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_DEV_VER);

	m_DevLocalPort = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_DEV_LOCAL_PORT);
	m_DevDestPort = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_DEST_PORT);
	m_DevName = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_DEV_NAME);
	m_DestIP = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_DEST_IP);

	m_dev_gap_time = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_GAP_TIME);
	m_dev_packing_len = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_PACKING_LEN);

	// 设备支持的功能
	m_FuncSelModbusTCP = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_FUNC_MODBUS);
	m_FuncSel2P2p = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_FUNC_P2P);

	// 功能选择
	m_EnableP2p = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_ENABLE_P2P);
	m_EnableSendMac = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_ENABLE_SEND_MAC);
	m_EnableTCPNeedKey = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_ENABLE_TCP_NEED_KEY);
	m_UDPModifyNeedKey = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_ENABLE_UDP_MODIFY_NEED_KEY);

	// 485多主机
	m_MultiHostTimeOut = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_485_TIME_OUT);
	m_485HalfGap = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_485_GAP);

	// Wifi参数
	m_WifiSSID = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_WIFI_SSID);
	m_WifiKey = (*m_pZLDM_GetDevParamString)(g_pCurSelID, PARAM_WIFI_KEY);

	m_WifiDHCPServer.InsertString(-1,"Disable");
	m_WifiDHCPServer.InsertString(-1,"Enable");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_WIFI_DHCP_SERVER);
	m_WifiDHCPServer.SetCurSel(value);
	
	m_WifiEthWifiBridge.InsertString(-1,"Disable");
	m_WifiEthWifiBridge.InsertString(-1,"Enable");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_WIFI_ETH_BRIDGE);
	m_WifiEthWifiBridge.SetCurSel(value);

	m_WifiKeyType.InsertString(-1,"NONE");
	m_WifiKeyType.InsertString(-1,"WEB64");
	m_WifiKeyType.InsertString(-1,"WEB128");
	m_WifiKeyType.InsertString(-1,"TKIP");
	m_WifiKeyType.InsertString(-1,"AES");
	m_WifiKeyType.InsertString(-1,"AUTO");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_WIFI_KEY_TYPE);
	m_WifiKeyType.SetCurSel(value);

	m_WifiSTAAP.InsertString(-1,"AP");
	m_WifiSTAAP.InsertString(-1,"STA");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_WIFI_STA_AP);
	m_WifiSTAAP.SetCurSel(value);

	m_WifiChannel.InsertString(-1,"1");
	m_WifiChannel.InsertString(-1,"2");
	m_WifiChannel.InsertString(-1,"3");
	m_WifiChannel.InsertString(-1,"4");
	m_WifiChannel.InsertString(-1,"5");
	m_WifiChannel.InsertString(-1,"6");
	m_WifiChannel.InsertString(-1,"7");
	m_WifiChannel.InsertString(-1,"8");
	m_WifiChannel.InsertString(-1,"9");
	m_WifiChannel.InsertString(-1,"10");
	m_WifiChannel.InsertString(-1,"11");
	value = (*m_pZLDM_GetDevParamInt)(g_pCurSelID, PARAM_WIFI_CHANNEL);
	m_WifiChannel.SetCurSel(value-1);

	UpdateData(FALSE);

	return TRUE;  // return TRUE unless you set the focus to a control
	              // EXCEPTION: OCX Property Pages should return FALSE
}


void CDlgParam::OnDefParam() 
{
	(*m_pZLDM_SetDevParamInt)(g_pCurSelID, 0, PARAM_SET_TO_DEFAULT);

	// 执行以上的写入的结果
	(*m_pZLDM_SetDevParamExcute)(g_pCurSelID);
	CDialog::OnOK();
}

void CDlgParam::OnSelchangeDevAppProtocol() 
{
	// 485多主机，非modbus方式一般不需要设置485的2个参数，设置为会产生其它不想要的效果。
	if(m_DevAppProtocol.GetCurSel() == 0)	// NONE
	{
		m_MultiHostTimeOut = 0;
		m_485HalfGap = 0;
		UpdateData(FALSE);
	}	
}
