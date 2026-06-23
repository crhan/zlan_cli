#if !defined(AFX_DLGPARAM_H__DC2F0376_6B80_42E8_A184_09B6610CD446__INCLUDED_)
#define AFX_DLGPARAM_H__DC2F0376_6B80_42E8_A184_09B6610CD446__INCLUDED_

#if _MSC_VER > 1000
#pragma once
#endif // _MSC_VER > 1000
// DlgParam.h : header file
//
/////////////////////////////////////////////////////////////////////////////
// CDlgParam dialog

class CDlgParam : public CDialog
{
// Construction
public:
	CDlgParam(CWnd* pParent = NULL);   // standard constructor
	void OnOK();
// Dialog Data
	//{{AFX_DATA(CDlgParam)
	enum { IDD = IDD_PARAM };
	CComboBox	m_WifiChannel;
	CComboBox	m_WifiDHCPServer;
	CComboBox	m_WifiEthWifiBridge;
	CComboBox	m_WifiKeyType;
	CComboBox	m_WifiSTAAP;
	CComboBox	m_DevAppProtocol;
	CComboBox	m_DevDestMode;
	CComboBox	m_DevFlowControl;
	CComboBox	m_DevIPMode;
	CComboBox	m_DevWorkMode;
	CComboBox	m_DevStopBit;
	CComboBox	m_DevParity;
	CComboBox	m_DevBitNum;
	CComboBox	m_DevBaundrate;
	CString	m_DevName;
	UINT	m_DevDestPort;
	UINT	m_dev_gap_time;
	UINT	m_DevLocalPort;
	UINT	m_dev_packing_len;
	CString	m_DestIP;
	UINT	m_ReconnectTime;
	UINT	m_KeepAliveTime;
	UINT	m_WebPort;
	CString	m_dev_ver;
	CString	m_DevGateway;
	CString	m_DevLocalIP;
	CString	m_DnsServerIP;
	CString	m_DevUDPGroupIP;
	CString	m_DevNetmask;
	UINT	m_485HalfGap;
	BOOL	m_EnableP2p;
	BOOL	m_EnableSendMac;
	UINT	m_MultiHostTimeOut;
	BOOL	m_FuncSelModbusTCP;
	BOOL	m_FuncSel2P2p;
	BOOL	m_EnableTCPNeedKey;
	BOOL	m_UDPModifyNeedKey;
	CString	m_InputKey;
	CString	m_WifiSSID;
	CString	m_WifiKey;
	//}}AFX_DATA


// Overrides
	// ClassWizard generated virtual function overrides
	//{{AFX_VIRTUAL(CDlgParam)
	protected:
	virtual void DoDataExchange(CDataExchange* pDX);    // DDX/DDV support
	//}}AFX_VIRTUAL

// Implementation
protected:

	// Generated message map functions
	//{{AFX_MSG(CDlgParam)
	virtual BOOL OnInitDialog();
	afx_msg void OnDefParam();
	afx_msg void OnSelchangeDevAppProtocol();
	//}}AFX_MSG
	DECLARE_MESSAGE_MAP()
};

//{{AFX_INSERT_LOCATION}}
// Microsoft Visual C++ will insert additional declarations immediately before the previous line.

#endif // !defined(AFX_DLGPARAM_H__DC2F0376_6B80_42E8_A184_09B6610CD446__INCLUDED_)
