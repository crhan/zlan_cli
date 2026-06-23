// ZLUseDevManageDlg.cpp : implementation file
//

#include "stdafx.h"
#include "ZLUseDevManage.h"
#include "DlgParam.h"
#include "ZLUseDevManageDlg.h"
#include "ManualIP.h"
#include "P2pManage.h"
#include "DlgConnectWithKey.h"

#ifdef _DEBUG
#define new DEBUG_NEW
#undef THIS_FILE
static char THIS_FILE[] = __FILE__;
#endif

HINSTANCE m_hZLDevManage;
tZLDM_Init				m_pZLDM_Init;
tZLDM_Exit				m_pZLDM_Exit;
tZLDM_StartSearchDev	m_pZLDM_StartSearchDev;
tZLDM_GetDevID	m_pZLDM_GetDevID;
tZLDM_GetDevParamString		m_pZLDM_GetDevParamString;
tZLDM_GetDevParamInt		m_pZLDM_GetDevParamInt;
tZLDM_SetDevParamString		m_pZLDM_SetDevParamString;
tZLDM_SetDevParamInt		m_pZLDM_SetDevParamInt;
tZLDM_SetDevParamExcute		m_pZLDM_SetDevParamExcute;
tZLDM_ModifyKey				m_pZLDM_ModifyKey;
tZLDM_GetKeyCmdType				m_pZLDM_GetKeyCmdType;
tZLDM_Random				m_pZLDM_Random;
tZLDM_Crypt					m_pZLDM_Crypt;
tZLDM_GetVer		m_pZLDM_GetVer;
tZLDM_AddManualDev	m_pZLDM_AddManualDev;
tZLDM_ClearManualDev		m_pZLDM_ClearManualDev;

tZLDM_P2pOpen 	m_pZLDM_P2pOpen;
tZLDM_P2pClose 	m_pZLDM_P2pClose;
tZLDM_P2pAddPort  m_pZLDM_P2pAddPort;
tZLDM_P2pDelPort  m_pZLDM_P2pDelPort;
tZLDM_P2pRestartAll  m_pZLDM_P2pRestartAll;
tZLDM_P2pGetID  m_pZLDM_P2pGetID;
tZLDM_P2pRetartPort  m_pZLDM_P2pRetartPort;

CString m_h[ZLDM_HANDLER_ARRAY_MIN_SIZE];
int m_DevCnt;

// 当前正在编辑的设备id
CString g_pCurSelID;


/////////////////////////////////////////////////////////////////////////////
// CAboutDlg dialog used for App About

class CAboutDlg : public CDialog
{
public:
	CAboutDlg();

// Dialog Data
	//{{AFX_DATA(CAboutDlg)
	enum { IDD = IDD_ABOUTBOX };
	CString	m_DllVer;
	//}}AFX_DATA

	// ClassWizard generated virtual function overrides
	//{{AFX_VIRTUAL(CAboutDlg)
	protected:
	virtual void DoDataExchange(CDataExchange* pDX);    // DDX/DDV support
	//}}AFX_VIRTUAL

// Implementation
protected:
	//{{AFX_MSG(CAboutDlg)
	//}}AFX_MSG
	DECLARE_MESSAGE_MAP()
};

CAboutDlg::CAboutDlg() : CDialog(CAboutDlg::IDD)
{
	//{{AFX_DATA_INIT(CAboutDlg)
	m_DllVer = _T("");
	//}}AFX_DATA_INIT
}

void CAboutDlg::DoDataExchange(CDataExchange* pDX)
{
	CDialog::DoDataExchange(pDX);
	//{{AFX_DATA_MAP(CAboutDlg)
	DDX_Text(pDX, IDC_DLL_VER, m_DllVer);
	//}}AFX_DATA_MAP
}

BEGIN_MESSAGE_MAP(CAboutDlg, CDialog)
	//{{AFX_MSG_MAP(CAboutDlg)
		// No message handlers
	//}}AFX_MSG_MAP
END_MESSAGE_MAP()

/////////////////////////////////////////////////////////////////////////////
// CZLUseDevManageDlg dialog
CZLUseDevManageDlg::CZLUseDevManageDlg(CWnd* pParent /*=NULL*/)
	: CDialog(CZLUseDevManageDlg::IDD, pParent)
{
	//{{AFX_DATA_INIT(CZLUseDevManageDlg)
	//}}AFX_DATA_INIT
	// Note that LoadIcon does not require a subsequent DestroyIcon in Win32
	m_hIcon = AfxGetApp()->LoadIcon(IDR_MAINFRAME);
}

CZLUseDevManageDlg::~CZLUseDevManageDlg()
{
	(*m_pZLDM_Exit)();
}

void CZLUseDevManageDlg::DoDataExchange(CDataExchange* pDX)
{
	CDialog::DoDataExchange(pDX);
	//{{AFX_DATA_MAP(CZLUseDevManageDlg)
	DDX_Control(pDX, IDC_DEV_LIST, m_DevListCtrl);
	//}}AFX_DATA_MAP
}

BEGIN_MESSAGE_MAP(CZLUseDevManageDlg, CDialog)
	//{{AFX_MSG_MAP(CZLUseDevManageDlg)
	ON_WM_SYSCOMMAND()
	ON_WM_PAINT()
	ON_WM_QUERYDRAGICON()
	ON_BN_CLICKED(ID_START_SEARCH, OnStartSearch)
	ON_BN_CLICKED(ID_PARAM_EDIT, OnParamEdit)
	ON_NOTIFY(NM_DBLCLK, IDC_DEV_LIST, OnDblclkDevList)
	ON_BN_CLICKED(ID_MANUAL_ADD, OnManualAdd)
	ON_BN_CLICKED(ID_ABOUT, OnAbout)
	ON_BN_CLICKED(ID_P2P_MANAGE, OnP2pManage)
	ON_BN_CLICKED(ID_SAFE_CONNECT, OnSafeConnect)
	//}}AFX_MSG_MAP
END_MESSAGE_MAP()

/////////////////////////////////////////////////////////////////////////////
// CZLUseDevManageDlg message handlers
BOOL CZLUseDevManageDlg::OnInitDialog()
{
	CString str2;
	CDialog::OnInitDialog();

	// Add "About..." menu item to system menu.

	// IDM_ABOUTBOX must be in the system command range.
	ASSERT((IDM_ABOUTBOX & 0xFFF0) == IDM_ABOUTBOX);
	ASSERT(IDM_ABOUTBOX < 0xF000);

	CMenu* pSysMenu = GetSystemMenu(FALSE);
	if (pSysMenu != NULL)
	{
		CString strAboutMenu;
		strAboutMenu.LoadString(IDS_ABOUTBOX);
		if (!strAboutMenu.IsEmpty())
		{
			pSysMenu->AppendMenu(MF_SEPARATOR);
			pSysMenu->AppendMenu(MF_STRING, IDM_ABOUTBOX, strAboutMenu);
		}
	}

	// Set the icon for this dialog.  The framework does this automatically
	//  when the application's main window is not a dialog
	SetIcon(m_hIcon, TRUE);			// Set big icon
	SetIcon(m_hIcon, FALSE);		// Set small icon
	
	// TODO: Add extra initialization here

	// 设备列表初始化
	m_DevListCtrl.SetExtendedStyle(m_DevListCtrl.GetExtendedStyle() 
		| LVS_EX_FULLROWSELECT //支持整行选择
		//| LVS_EX_GRIDLINES		//显示网格
		//| LVS_EX_HEADERDRAGDROP	// 支持列的拖动		
	);
	str2.LoadString(ZL_STR_DEV_STATUS);
	m_DevListCtrl.InsertColumn(DEV_COL_STATUS, str2, LVCFMT_LEFT, 100);
	str2.LoadString(ZL_STR_DEV_NAME);
	m_DevListCtrl.InsertColumn(DEV_COL_NAME, str2, LVCFMT_LEFT, 120);
	str2.LoadString(ZL_STR_DEV_IP);
	m_DevListCtrl.InsertColumn(DEV_COL_IP, str2, LVCFMT_LEFT, 150);
	str2.LoadString(ZL_STR_DEV_PORT);
	m_DevListCtrl.InsertColumn(DEV_COL_PORT, str2, LVCFMT_LEFT, 120);
	str2.LoadString(ZL_STR_DEV_ID);
	m_DevListCtrl.InsertColumn(DEV_COL_ID, str2, LVCFMT_LEFT, 150);
	//在用户程序初始化函数（例如OnInitDialog）中，加载DLL，并获取函数。

	m_hZLDevManage = LoadLibrary("ZLDevManage.dll");
	m_pZLDM_Init	= (tZLDM_Init)GetProcAddress(m_hZLDevManage,"ZLDM_Init");
	m_pZLDM_Exit	= (tZLDM_Exit)GetProcAddress(m_hZLDevManage,"ZLDM_Exit");
	m_pZLDM_StartSearchDev = (tZLDM_StartSearchDev)GetProcAddress(m_hZLDevManage,"ZLDM_StartSearchDev");
	m_pZLDM_GetDevID = (tZLDM_GetDevID)GetProcAddress(m_hZLDevManage,"ZLDM_GetDevID");
	
	m_pZLDM_GetDevParamString = (tZLDM_GetDevParamString)GetProcAddress(m_hZLDevManage, "ZLDM_GetDevParamString");
	m_pZLDM_GetDevParamInt = (tZLDM_GetDevParamInt)GetProcAddress(m_hZLDevManage, "ZLDM_GetDevParamInt");
	m_pZLDM_SetDevParamString = (tZLDM_SetDevParamString)GetProcAddress(m_hZLDevManage, "ZLDM_SetDevParamString");
	m_pZLDM_SetDevParamInt = (tZLDM_SetDevParamInt)GetProcAddress(m_hZLDevManage, "ZLDM_SetDevParamInt");
	m_pZLDM_SetDevParamExcute = (tZLDM_SetDevParamExcute)GetProcAddress(m_hZLDevManage, "ZLDM_SetDevParamExcute");
	m_pZLDM_ModifyKey = (tZLDM_ModifyKey)GetProcAddress(m_hZLDevManage, "ZLDM_ModifyKey");
	m_pZLDM_GetKeyCmdType = (tZLDM_GetKeyCmdType)GetProcAddress(m_hZLDevManage, "ZLDM_GetKeyCmdType");
	m_pZLDM_Random = (tZLDM_Random)GetProcAddress(m_hZLDevManage, "ZLDM_Random");
	m_pZLDM_Crypt = (tZLDM_Crypt)GetProcAddress(m_hZLDevManage, "ZLDM_Crypt");
	m_pZLDM_GetVer = (tZLDM_GetVer)GetProcAddress(m_hZLDevManage,"ZLDM_GetVer");
	m_pZLDM_AddManualDev = (tZLDM_AddManualDev)GetProcAddress(m_hZLDevManage,"ZLDM_AddManualDev");
	m_pZLDM_ClearManualDev = (tZLDM_ClearManualDev)GetProcAddress(m_hZLDevManage,"ZLDM_ClearManualDev");

	m_pZLDM_P2pOpen = (tZLDM_P2pOpen)GetProcAddress(m_hZLDevManage, "ZLDM_P2pOpen");
	m_pZLDM_P2pClose = (tZLDM_P2pClose)GetProcAddress(m_hZLDevManage, "ZLDM_P2pClose");
	m_pZLDM_P2pAddPort = (tZLDM_P2pAddPort)GetProcAddress(m_hZLDevManage, "ZLDM_P2pAddPort");
	m_pZLDM_P2pDelPort = (tZLDM_P2pDelPort)GetProcAddress(m_hZLDevManage, "ZLDM_P2pDelPort");
	m_pZLDM_P2pRestartAll = (tZLDM_P2pRestartAll)GetProcAddress(m_hZLDevManage, "ZLDM_P2pRestartAll");
	m_pZLDM_P2pGetID = (tZLDM_P2pGetID)GetProcAddress(m_hZLDevManage, "ZLDM_P2pGetID");
	m_pZLDM_P2pRetartPort = (tZLDM_P2pRetartPort)GetProcAddress(m_hZLDevManage, "ZLDM_P2pRetartPort");


	if(m_pZLDM_GetVer == NULL)
	{
		str2.LoadString(ZL_STR_DLL_LOAD_FAIL);
		AfxMessageBox(str2);
		return FALSE;
	}

	// 检测版本
	if(strcmp((*m_pZLDM_GetVer)(), ZLDM_VER) != 0)
	{
		str2.LoadString(ZL_STR_DLL_VER_EER);
		AfxMessageBox(str2);
	}

	// 初始化
	// 如果传入0之后StartParamListen没有被调用，则无法使用listensock接收param
	if((*m_pZLDM_Init)(4196) == FALSE)
	{
		str2.LoadString(ZL_STR_DLL_INIT_FAIL);
		AfxMessageBox(str2);
		return FALSE;
	}

	return TRUE;  // return TRUE  unless you set the focus to a control
}

void CZLUseDevManageDlg::OnSysCommand(UINT nID, LPARAM lParam)
{
	if ((nID & 0xFFF0) == IDM_ABOUTBOX)
	{
		CAboutDlg dlgAbout;
		dlgAbout.m_DllVer = (*m_pZLDM_GetVer)();
		dlgAbout.DoModal();
	}
	else
	{
		CDialog::OnSysCommand(nID, lParam);
	}
}

// If you add a minimize button to your dialog, you will need the code below
//  to draw the icon.  For MFC applications using the document/view model,
//  this is automatically done for you by the framework.

void CZLUseDevManageDlg::OnPaint() 
{
	if (IsIconic())
	{
		CPaintDC dc(this); // device context for painting

		SendMessage(WM_ICONERASEBKGND, (WPARAM) dc.GetSafeHdc(), 0);

		// Center icon in client rectangle
		int cxIcon = GetSystemMetrics(SM_CXICON);
		int cyIcon = GetSystemMetrics(SM_CYICON);
		CRect rect;
		GetClientRect(&rect);
		int x = (rect.Width() - cxIcon + 1) / 2;
		int y = (rect.Height() - cyIcon + 1) / 2;

		// Draw the icon
		dc.DrawIcon(x, y, m_hIcon);
	}
	else
	{
		CDialog::OnPaint();
	}
}

// The system calls this to obtain the cursor to display while the user drags
//  the minimized window.
HCURSOR CZLUseDevManageDlg::OnQueryDragIcon()
{
	return (HCURSOR) m_hIcon;
}

void CZLUseDevManageDlg::OnStartSearch() 
{
	CString str, str2, CurSelID;
	int i, value;

	m_DevCnt = (*m_pZLDM_StartSearchDev)();

	// 防止越界
	if(m_DevCnt > ZLDM_HANDLER_ARRAY_MIN_SIZE)
		return;
		
	// 逐个获得id
	for(i = 0; i < m_DevCnt; i++)
	{
		m_h[i] = (*m_pZLDM_GetDevID)(i);
	}

	// 添加

	m_DevListCtrl.DeleteAllItems();
	for(i = 0; i < m_DevCnt; i++)
	{
		m_DevListCtrl.InsertItem(i, "1");
		value = (*m_pZLDM_GetDevParamInt)((LPCSTR)m_h[i], PARAM_LINK_STATUS);
		if(value == 0)
		{
			// link
			str2.LoadString(ZL_STR_NOT_LINKED);
			m_DevListCtrl.SetItemText(i, DEV_COL_STATUS, str2);

		}
		else
		{
			str2.LoadString(ZL_STR_LINKED);
			m_DevListCtrl.SetItemText(i, DEV_COL_STATUS, str2);			
		}
		
		// dev name
		str = (*m_pZLDM_GetDevParamString)(m_h[i], PARAM_DEV_NAME);
		m_DevListCtrl.SetItemText(i, DEV_COL_NAME, str);

		// ip
		str = (*m_pZLDM_GetDevParamString)(m_h[i], PARAM_DEV_LOCAL_IP);
		m_DevListCtrl.SetItemText(i, DEV_COL_IP, str);

		// port
		value = (*m_pZLDM_GetDevParamInt)(m_h[i], PARAM_DEV_LOCAL_PORT);
		str.Format("%d", value);
		m_DevListCtrl.SetItemText(i, DEV_COL_PORT, str);
		
		m_DevListCtrl.SetItemText(i, DEV_COL_ID, m_h[i]);
	}
	if(m_DevCnt >= 1)
	{
		m_DevListCtrl.SetFocus();
		m_DevListCtrl.SetItemState(0, LVIS_FOCUSED | LVIS_SELECTED,
			LVIS_FOCUSED | LVIS_SELECTED); 
	}
}

void CZLUseDevManageDlg::OnParamEdit() 
{
	CDlgParam dlg;

	g_pCurSelID = m_DevListCtrl.GetItemText(GetCurSel(), DEV_COL_ID);
	if(dlg.DoModal() == IDOK)
	{
	}
	OnStartSearch();
}

int CZLUseDevManageDlg::GetCurSel()
{
	POSITION pos;

	pos = m_DevListCtrl.GetFirstSelectedItemPosition();
	return m_DevListCtrl.GetNextSelectedItem(pos);
}

void CZLUseDevManageDlg::OnDblclkDevList(NMHDR* pNMHDR, LRESULT* pResult) 
{
	OnParamEdit();	
	*pResult = 0;
}

void CZLUseDevManageDlg::OnManualAdd() 
{
	CManualIP dlg;
	dlg.DoModal();
	OnStartSearch();
}


void CZLUseDevManageDlg::OnAbout() 
{
	OnSysCommand(IDM_ABOUTBOX, 0);
}

void CZLUseDevManageDlg::OnP2pManage() 
{
	CP2pManage dlg;
	dlg.DoModal();
}

void CZLUseDevManageDlg::OnSafeConnect() 
{
	CDlgConnectWithKey dlg;
	dlg.DoModal();
}
