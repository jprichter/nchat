// Package testcallbacks supplies C++ callbacks only to Go bridge tests.
package testcallbacks

/*
void WmNewContactsNotify(int p_ConnId, char* p_ChatId, char* p_Name, char* p_Phone, int p_IsSelf, int p_IsAlias, int p_Notify) { }
void WmNewChatsNotify(int p_ConnId, char* p_ChatId, int p_IsUnread, int p_IsMuted, int p_IsPinned, int p_IsArchived, int p_LastMessageTime) { }
void WmNewGroupMembersNotify(int p_ConnId, char* p_ChatId, char* p_MembersJson) { }
void WmNewMessagesNotify(int p_ConnId, char* p_ChatId, char* p_MsgId, char* p_SenderId, char* p_Text, int p_FromMe, char* p_QuotedId, char* p_FileId, char* p_FilePath, int p_FileStatus, int p_TimeSent, int p_IsRead, int p_IsEdited) { }
void WmNewStatusNotify(int p_ConnId, char* p_UserId, int p_IsOnline, int p_TimeSeen) { }
void WmNewTypingNotify(int p_ConnId, char* p_ChatId, char* p_UserId, int p_IsTyping) { }
void WmNewMessageStatusNotify(int p_ConnId, char* p_ChatId, char* p_MsgId, int p_IsRead) { }
void WmNewMessageIsPinnedNotify(int p_ConnId, char* p_ChatId, char* p_MsgId, int p_IsPinned) { }
void WmNewMessageFileNotify(int p_ConnId, char* p_ChatId, char* p_MsgId, char* p_FileId, char* p_FilePath, int p_FileStatus, int p_Action) { }
void WmNewMessageReactionNotify(int p_ConnId, char* p_ChatId, char* p_MsgId, char* p_SenderId, char* p_Text, int p_FromMe) { }
void WmDeleteChatNotify(int p_ConnId, char* p_ChatId) { }
void WmDeleteMessageNotify(int p_ConnId, char* p_ChatId, char* p_MsgId, int p_IsOutgoing) { }
void WmUpdateMuteNotify(int p_ConnId, char* p_ChatId, int p_IsMuted) { }
void WmUpdateArchivedNotify(int p_ConnId, char* p_ChatId, int p_IsArchived) { }
void WmUpdatePinNotify(int p_ConnId, char* p_ChatId, int p_IsPinned, int p_TimePinned) { }
void WmReinit(int p_ConnId) { }
void WmSetProtocolUiControl(int p_ConnId, int p_IsTakeControl) { }
void WmSetStatus(int p_ConnId, int p_Flags) { }
void WmClearStatus(int p_ConnId, int p_Flags) { }
int WmAppConfigGetNum(char* p_Param) { return 0; }
void WmAppConfigSetNum(char* p_Param, int p_Value) { }
void WmLogTrace(char* p_Filename, int p_LineNo, char* p_Message) { }
void WmLogDebug(char* p_Filename, int p_LineNo, char* p_Message) { }
void WmLogInfo(char* p_Filename, int p_LineNo, char* p_Message) { }
void WmLogWarning(char* p_Filename, int p_LineNo, char* p_Message) { }
void WmLogError(char* p_Filename, int p_LineNo, char* p_Message) { }
*/
import "C"
